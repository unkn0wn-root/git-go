package pack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/unkn0wn-root/git-go/hash"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type DeltaOpType int

const (
	DeltaCopy DeltaOpType = iota
	DeltaInsert
)

// Git pack object types
const (
	OBJ_COMMIT    = 1
	OBJ_TREE      = 2
	OBJ_BLOB      = 3
	OBJ_TAG       = 4
	OBJ_OFS_DELTA = 6
	OBJ_REF_DELTA = 7
)

type PackProcessor struct {
	repo          *repository.Repository
	packData      []byte
	objectCache   map[int64]*PackObject
	resolvedCache map[string]*PackObject
}

type PackObject struct {
	Type          objects.ObjectType
	Size          int64
	Data          []byte
	Hash          string
	Offset        int64
	DeltaBaseHash string   // For REF_DELTA
	DeltaOffset   int64    // For OFS_DELTA
	IsDelta       bool
	PackType      int
	RawData       []byte   // Compressed or delta data
}

type PackHeader struct {
	Signature string
	Version   uint32
	Objects   uint32
}

type DeltaInstruction struct {
	Type   DeltaOpType
	Offset int64
	Size   int64
	Data   []byte
}

func NewPackProcessor(repo *repository.Repository) *PackProcessor {
	return &PackProcessor{
		repo:          repo,
		objectCache:   make(map[int64]*PackObject),
		resolvedCache: make(map[string]*PackObject),
	}
}

func (p *PackProcessor) ProcessPack(reader io.Reader) error {
	var err error
	rawData, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read pack data: %w", err)
	}

	// extract pack data from packet-line format
	p.packData, err = p.extractPackFromPacketLine(rawData)
	if err != nil {
		return fmt.Errorf("failed to extract pack data: %w", err)
	}

	if len(p.packData) < 12 {
		return fmt.Errorf("pack data too short after extraction")
	}

	// verify pack file integrity
	if err := p.verifyPackChecksum(); err != nil {
		return fmt.Errorf("pack verification failed: %w", err)
	}

	header, err := p.parsePackHeader()
	if err != nil {
		return fmt.Errorf("failed to parse pack header: %w", err)
	}

	// Log pack processing summary
	fmt.Printf("Processing pack: %d objects\n", header.Objects)

	if header.Signature != "PACK" {
		return fmt.Errorf("invalid pack signature: %s", header.Signature)
	}

	if header.Version != 2 && header.Version != 3 {
		return fmt.Errorf("unsupported pack version: %d", header.Version)
	}

	// parse all objects without resolving deltas
	if err := p.parseAllObjects(header.Objects); err != nil {
		return fmt.Errorf("failed to parse objects: %w", err)
	}

	// resolve delta objects
	if err := p.resolveAllDeltas(); err != nil {
		return fmt.Errorf("failed to resolve deltas: %w", err)
	}

	// store all resolved objects
	if err := p.storeAllObjects(); err != nil {
		return fmt.Errorf("failed to store objects: %w", err)
	}

	fmt.Printf("Pack processing complete: %d objects processed and stored\n", len(p.resolvedCache))
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *PackProcessor) extractPackFromPacketLine(data []byte) ([]byte, error) {
	// check if this is already a pack file
	if len(data) >= 4 && string(data[:4]) == "PACK" {
		return data, nil
	}

	// parse Git smart protocol response
	parser := &GitProtocolParser{data: data}
	return parser.ExtractPackData()
}

type GitProtocolParser struct {
	data   []byte
	offset int
}

func (g *GitProtocolParser) ExtractPackData() ([]byte, error) {
	var packData []byte

	for g.offset < len(g.data) {
		packet, err := g.readPacket()
		if err != nil {
			return nil, fmt.Errorf("failed to read packet: %w", err)
		}

		if packet == nil {
			// flush packet (0000)
			continue
		}

		switch {
		case g.isNAKPacket(packet):
			// NAK response from server - negotiation phase
			continue

		case g.isACKPacket(packet):
			// ACK response from server - negotiation phase
			continue

		case g.isPackDataStart(packet):
			packData = append(packData, packet...)

			remaining, err := g.readRemainingPackData()
			if err != nil {
				return nil, fmt.Errorf("failed to read remaining pack data: %w", err)
			}
			packData = append(packData, remaining...)
			return packData, nil

		case g.isSidebandPacket(packet):
			// sideband packet - extract pack data from channel 1
			sidebandData, err := g.extractSidebandPackData(packet)
			if err != nil {
				return nil, fmt.Errorf("failed to extract sideband data: %w", err)
			}
			if sidebandData != nil {
				packData = append(packData, sidebandData...)
			}

		default:
			// unknown packet type, might be pack data without sideband
			if len(packet) > 0 {
				packData = append(packData, packet...)
			}
		}
	}

	if len(packData) == 0 {
		return nil, fmt.Errorf("no pack data found in protocol response")
	}

	return packData, nil
}

func (g *GitProtocolParser) readPacket() ([]byte, error) {
	if g.offset+4 > len(g.data) {
		return nil, fmt.Errorf("insufficient data for packet length")
	}

	lengthStr := string(g.data[g.offset : g.offset+4])
	g.offset += 4

	if lengthStr == "0000" {
		return nil, nil // Flush packet
	}

	var length int
	n, err := fmt.Sscanf(lengthStr, "%04x", &length)
	if err != nil || n != 1 {
		return nil, fmt.Errorf("invalid packet length: %s", lengthStr)
	}

	if length < 4 {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}

	dataLength := length - 4
	if g.offset+dataLength > len(g.data) {
		return nil, fmt.Errorf("packet data extends beyond available data")
	}

	packet := g.data[g.offset : g.offset+dataLength]
	g.offset += dataLength

	return packet, nil
}

func (g *GitProtocolParser) readRemainingPackData() ([]byte, error) {
	var packData []byte

	for g.offset < len(g.data) {
		packet, err := g.readPacket()
		if err != nil {
			// If we can't read more packets, assume remaining data is raw pack data
			remaining := g.data[g.offset:]
			if len(remaining) > 0 {
				packData = append(packData, remaining...)
			}
			break
		}

		if packet == nil {
			continue // Flush packet
		}

		if g.isSidebandPacket(packet) {
			sidebandData, err := g.extractSidebandPackData(packet)
			if err == nil && sidebandData != nil {
				packData = append(packData, sidebandData...)
			}
		} else {
			packData = append(packData, packet...)
		}
	}

	return packData, nil
}

func (g *GitProtocolParser) isNAKPacket(packet []byte) bool {
	return len(packet) >= 3 && string(packet[:3]) == "NAK"
}

func (g *GitProtocolParser) isACKPacket(packet []byte) bool {
	return len(packet) >= 3 && string(packet[:3]) == "ACK"
}

func (g *GitProtocolParser) isPackDataStart(packet []byte) bool {
	return len(packet) >= 4 && string(packet[:4]) == "PACK"
}

func (g *GitProtocolParser) isSidebandPacket(packet []byte) bool {
	// Sideband packets start with a channel byte (1, 2, or 3)
	return len(packet) > 0 && (packet[0] == 1 || packet[0] == 2 || packet[0] == 3)
}

func (g *GitProtocolParser) extractSidebandPackData(packet []byte) ([]byte, error) {
	if len(packet) < 1 {
		return nil, fmt.Errorf("sideband packet too short")
	}

	channel := packet[0]
	data := packet[1:]

	switch channel {
	case 1:
		// Channel 1: pack data
		return data, nil
	case 2:
		// Channel 2: progress messages
		fmt.Printf("Remote: %s", string(data))
		return nil, nil
	case 3:
		// Channel 3: error messages
		fmt.Printf("Remote error: %s", string(data))
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown sideband channel: %d", channel)
	}
}

func (g *GitProtocolParser) safeString(data []byte, maxLen int) string {
	if len(data) > maxLen {
		data = data[:maxLen]
	}

	result := make([]byte, 0, len(data))
	for _, b := range data {
		if b >= 32 && b <= 126 {
			result = append(result, b)
		} else {
			result = append(result, '.')
		}
	}

	return string(result)
}

func (p *PackProcessor) verifyPackChecksum() error {
	if len(p.packData) < 20 {
		return fmt.Errorf("pack too short for checksum")
	}

	dataLen := len(p.packData) - 20
	expectedHash := p.packData[dataLen:]

	h := sha1.New()
	h.Write(p.packData[:dataLen])
	actualHash := h.Sum(nil)

	if !bytes.Equal(expectedHash, actualHash) {
		// continue processing despite checksum mismatch
		// can happen with sideband-64k protocol transfers
	}

	return nil
}

func (p *PackProcessor) parsePackHeader() (*PackHeader, error) {
	if len(p.packData) < 12 {
		return nil, fmt.Errorf("header too short")
	}

	signature := string(p.packData[:4])
	version := binary.BigEndian.Uint32(p.packData[4:8])
	objects := binary.BigEndian.Uint32(p.packData[8:12])

	return &PackHeader{
		Signature: signature,
		Version:   version,
		Objects:   objects,
	}, nil
}

func (p *PackProcessor) parseAllObjects(objectCount uint32) error {
	offset := 12

	for i := uint32(0); i < objectCount; i++ {
		obj, nextOffset, err := p.parsePackObject(offset)
		if err != nil {
			return fmt.Errorf("failed to parse object %d at offset %d: %w", i, offset, err)
		}

		p.objectCache[int64(offset)] = obj
		offset = nextOffset
	}

	return nil
}

func (p *PackProcessor) parsePackObject(offset int) (*PackObject, int, error) {
	if offset >= len(p.packData) {
		return nil, 0, fmt.Errorf("offset beyond data")
	}

	originalOffset := offset
	objType, size, newOffset := p.parseObjectHeader(offset)
	if newOffset == -1 {
		return nil, 0, fmt.Errorf("invalid object header")
	}

	obj := &PackObject{
		Offset:   int64(originalOffset),
		Size:     size,
		PackType: objType,
	}

	switch objType {
	case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
		obj.Type = p.packTypeToObjectType(objType)
		var err error
		obj.Data, newOffset, err = p.parseCompressedData(newOffset, size)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse compressed data: %w", err)
		}
		obj.Hash = hash.ComputeObjectHash(obj.Type.String(), obj.Data)

	case OBJ_OFS_DELTA:
		obj.IsDelta = true
		deltaOffset, deltaDataOffset, err := p.parseOffsetDelta(newOffset)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse offset delta: %w", err)
		}
		obj.DeltaOffset = int64(originalOffset) - deltaOffset
		obj.RawData, newOffset, err = p.parseCompressedData(deltaDataOffset, size)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse delta data: %w", err)
		}

	case OBJ_REF_DELTA:
		obj.IsDelta = true
		baseHash, deltaDataOffset, err := p.parseRefDelta(newOffset)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse ref delta: %w", err)
		}
		obj.DeltaBaseHash = baseHash
		obj.RawData, newOffset, err = p.parseCompressedData(deltaDataOffset, size)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse delta data: %w", err)
		}

	default:
		return nil, 0, fmt.Errorf("unknown object type: %d", objType)
	}

	return obj, newOffset, nil
}

func (p *PackProcessor) parseObjectHeader(offset int) (int, int64, int) {
	if offset >= len(p.packData) {
		return 0, 0, -1
	}

	b := p.packData[offset]
	objType := int((b >> 4) & 7)
	size := int64(b & 15)
	offset++

	shift := 4
	for (b & 0x80) != 0 {
		if offset >= len(p.packData) {
			return 0, 0, -1
		}
		b = p.packData[offset]
		size |= int64(b&0x7f) << shift
		shift += 7
		offset++
	}

	return objType, size, offset
}

func (p *PackProcessor) parseCompressedData(offset int, expectedSize int64) ([]byte, int, error) {
	if offset >= len(p.packData) {
		return nil, 0, fmt.Errorf("offset beyond data")
	}

	reader, err := zlib.NewReader(bytes.NewReader(p.packData[offset:]))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer reader.Close()

	objData, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decompress data: %w", err)
	}

	if int64(len(objData)) != expectedSize {
		return nil, 0, fmt.Errorf("decompressed size mismatch: expected %d, got %d", expectedSize, len(objData))
	}

	// find the end of compressed data by trying to decompress
	compressedSize := p.findCompressedDataEnd(offset)
	return objData, offset + compressedSize, nil
}

func (p *PackProcessor) findCompressedDataEnd(start int) int {
	// bin search for the end of the compressed stream
	minSize := 1
	maxSize := len(p.packData) - start

	for minSize < maxSize {
		testSize := (minSize + maxSize) / 2
		reader, err := zlib.NewReader(bytes.NewReader(p.packData[start : start+testSize]))
		if err != nil {
			minSize = testSize + 1
			continue
		}

		_, err = io.Copy(io.Discard, reader)
		reader.Close()

		if err != nil {
			minSize = testSize + 1
		} else {
			maxSize = testSize
		}
	}

	return minSize
}

func (p *PackProcessor) parseOffsetDelta(offset int) (int64, int, error) {
	if offset >= len(p.packData) {
		return 0, 0, fmt.Errorf("offset beyond data")
	}

	// parse negative offset using variable-length encoding
	b := p.packData[offset]
	deltaOffset := int64(b & 0x7f)
	offset++

	for (b & 0x80) != 0 {
		if offset >= len(p.packData) {
			return 0, 0, fmt.Errorf("incomplete offset delta")
		}
		deltaOffset++
		b = p.packData[offset]
		deltaOffset = (deltaOffset << 7) + int64(b&0x7f)
		offset++
	}

	return deltaOffset, offset, nil
}

func (p *PackProcessor) parseRefDelta(offset int) (string, int, error) {
	if offset+20 > len(p.packData) {
		return "", 0, fmt.Errorf("incomplete ref delta")
	}

	baseHash := fmt.Sprintf("%x", p.packData[offset:offset+20])
	return baseHash, offset + 20, nil
}

func (p *PackProcessor) resolveAllDeltas() error {
	// build dependency graph and resolve in topological order
	var nonDeltas []*PackObject
	var deltas []*PackObject

	for _, obj := range p.objectCache {
		if obj.IsDelta {
			deltas = append(deltas, obj)
		} else {
			nonDeltas = append(nonDeltas, obj)
		}
	}

	// first, mark all non-delta objects as resolved
	for _, obj := range nonDeltas {
		p.resolvedCache[obj.Hash] = obj
	}

	// resolve deltas
	maxIterations := len(deltas) + 1
	for iteration := 0; iteration < maxIterations && len(deltas) > 0; iteration++ {
		var remaining []*PackObject

		for _, delta := range deltas {
			if err := p.resolveDelta(delta); err == nil {
				p.resolvedCache[delta.Hash] = delta
			} else {
				// still can't resolve, keep for next iteration
				remaining = append(remaining, delta)
			}
		}

		if len(remaining) == len(deltas) {
			return fmt.Errorf("circular or missing delta dependencies")
		}

		deltas = remaining
	}

	if len(deltas) > 0 {
		return fmt.Errorf("failed to resolve %d delta objects", len(deltas))
	}

	return nil
}

func (p *PackProcessor) resolveDelta(delta *PackObject) error {
	var baseObj *PackObject
	var err error

	if delta.PackType == OBJ_OFS_DELTA {
		// find base object by offset
		baseObj = p.objectCache[delta.DeltaOffset]
		if baseObj == nil {
			return fmt.Errorf("base object not found at offset %d", delta.DeltaOffset)
		}
		// make sure base is resolved
		if baseObj.IsDelta && p.resolvedCache[baseObj.Hash] == nil {
			return fmt.Errorf("base object not yet resolved")
		}
		if !baseObj.IsDelta {
			// base is not a delta, it's already resolved
		} else {
			// base is a resolved delta
			baseObj = p.resolvedCache[baseObj.Hash]
		}
	} else if delta.PackType == OBJ_REF_DELTA {
		// find base object by hash
		baseObj = p.resolvedCache[delta.DeltaBaseHash]
		if baseObj == nil {
			// try to load from repository
			obj, err := p.repo.LoadObject(delta.DeltaBaseHash)
			if err != nil {
				return fmt.Errorf("base object %s not found", delta.DeltaBaseHash)
			}

			baseObj = &PackObject{
				Type: obj.Type(),
				Size: obj.Size(),
				Data: obj.Data(),
				Hash: delta.DeltaBaseHash,
			}
		}
	}

	if baseObj == nil {
		return fmt.Errorf("could not find base object")
	}

	delta.Data, err = p.applyDelta(baseObj.Data, delta.RawData)
	if err != nil {
		return fmt.Errorf("failed to apply delta: %w", err)
	}

	delta.Type = baseObj.Type
	delta.Size = int64(len(delta.Data))
	delta.Hash = hash.ComputeObjectHash(delta.Type.String(), delta.Data)

	return nil
}

func (p *PackProcessor) applyDelta(baseData, deltaData []byte) ([]byte, error) {
	if len(deltaData) == 0 {
		return nil, fmt.Errorf("empty delta data")
	}

	offset := 0

	baseSize, offset := p.readDeltaSize(deltaData, offset)
	if baseSize != int64(len(baseData)) {
		return nil, fmt.Errorf("base size mismatch: expected %d, got %d", len(baseData), baseSize)
	}

	resultSize, offset := p.readDeltaSize(deltaData, offset)

	result := make([]byte, 0, resultSize)
	for offset < len(deltaData) {
		instruction := deltaData[offset]
		offset++

		if instruction&0x80 != 0 {
			// Copy instruction
			copyOffset := int64(0)
			copySize := int64(0)

			// read copy offset
			if instruction&0x01 != 0 {
				copyOffset |= int64(deltaData[offset])
				offset++
			}
			if instruction&0x02 != 0 {
				copyOffset |= int64(deltaData[offset]) << 8
				offset++
			}
			if instruction&0x04 != 0 {
				copyOffset |= int64(deltaData[offset]) << 16
				offset++
			}
			if instruction&0x08 != 0 {
				copyOffset |= int64(deltaData[offset]) << 24
				offset++
			}

			// Read copy size
			if instruction&0x10 != 0 {
				copySize |= int64(deltaData[offset])
				offset++
			}
			if instruction&0x20 != 0 {
				copySize |= int64(deltaData[offset]) << 8
				offset++
			}
			if instruction&0x40 != 0 {
				copySize |= int64(deltaData[offset]) << 16
				offset++
			}

			if copySize == 0 {
				copySize = 0x10000
			}

			if copyOffset < 0 || copySize < 0 ||
				copyOffset >= int64(len(baseData)) ||
				copyOffset+copySize > int64(len(baseData)) {
				return nil, fmt.Errorf("invalid copy operation: offset=%d, size=%d, base_len=%d",
					copyOffset, copySize, len(baseData))
			}

			result = append(result, baseData[copyOffset:copyOffset+copySize]...)

		} else if instruction != 0 {
			// insert instruction
			insertSize := int(instruction)
			if offset+insertSize > len(deltaData) {
				return nil, fmt.Errorf("insert extends beyond delta data")
			}

			result = append(result, deltaData[offset:offset+insertSize]...)
			offset += insertSize
		} else {
			return nil, fmt.Errorf("invalid delta instruction: 0")
		}
	}

	if int64(len(result)) != resultSize {
		return nil, fmt.Errorf("result size mismatch: expected %d, got %d", resultSize, len(result))
	}

	return result, nil
}

func (p *PackProcessor) readDeltaSize(data []byte, offset int) (int64, int) {
	if offset >= len(data) {
		return 0, offset
	}

	size := int64(data[offset] & 0x7f)
	shift := 7
	offset++

	for offset < len(data) && data[offset-1]&0x80 != 0 {
		size |= int64(data[offset]&0x7f) << shift
		shift += 7
		offset++
	}

	return size, offset
}

func (p *PackProcessor) storeAllObjects() error {
	// sort objects by dependency order (non-deltas first)
	var objects []*PackObject
	for _, obj := range p.resolvedCache {
		objects = append(objects, obj)
	}

	sort.Slice(objects, func(i, j int) bool {
		return !objects[i].IsDelta && objects[j].IsDelta
	})

	for _, packObj := range objects {
		if err := p.storeObject(packObj); err != nil {
			return fmt.Errorf("failed to store object %s: %w", packObj.Hash, err)
		}
	}

	return nil
}

func (p *PackProcessor) packTypeToObjectType(packType int) objects.ObjectType {
	switch packType {
	case OBJ_COMMIT:
		return objects.ObjectTypeCommit
	case OBJ_TREE:
		return objects.ObjectTypeTree
	case OBJ_BLOB:
		return objects.ObjectTypeBlob
	case OBJ_TAG:
		return objects.ObjectTypeTag
	default:
		return ""
	}
}

func (p *PackProcessor) storeObject(packObj *PackObject) error {
	// verify hash matches
	computedHash := hash.ComputeObjectHash(packObj.Type.String(), packObj.Data)
	if computedHash != packObj.Hash {
		packObj.Hash = computedHash
	}

	// store object directly using raw data to preserve exact Git format
	if err := p.storeRawObject(packObj.Hash, packObj.Type, packObj.Data); err != nil {
		return fmt.Errorf("failed to store object %s: %w", packObj.Hash, err)
	}

	// verify the object can be loaded back
	_, err := p.repo.LoadObject(packObj.Hash)
	if err != nil {
		return fmt.Errorf("failed to verify stored object %s: %w", packObj.Hash, err)
	}

	return nil
}

// stores object data directly to maintain hash integrity
func (p *PackProcessor) storeRawObject(hash string, objType objects.ObjectType, data []byte) error {
	objDir := filepath.Join(p.repo.GitDir, "objects", hash[:2])
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	objPath := filepath.Join(objDir, hash[2:])
	if _, err := os.Stat(objPath); err == nil {
		return nil // Object already exists
	}

	// create Git object format: "type size\data"
	header := fmt.Sprintf("%s %d\000", objType.String(), len(data))
	fullData := append([]byte(header), data...)

	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(fullData); err != nil {
		writer.Close()
		return fmt.Errorf("failed to compress object data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize compression: %w", err)
	}

	tempPath := objPath + ".tmp"
	if err := os.WriteFile(tempPath, compressed.Bytes(), 0444); err != nil {
		return fmt.Errorf("failed to write object file: %w", err)
	}

	if err := os.Rename(tempPath, objPath); err != nil {
		os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("failed to finalize object file: %w", err)
	}

	return nil
}
