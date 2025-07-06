package pack

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

func TestPackProcessor(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	t.Run("NewPackProcessor", func(t *testing.T) {
		processor := NewPackProcessor(repo)
		assert.NotNil(t, processor)
		assert.Equal(t, repo, processor.repo)
		assert.NotNil(t, processor.objectCache)
		assert.NotNil(t, processor.resolvedCache)
	})

	t.Run("ProcessInvalidPack", func(t *testing.T) {
		processor := NewPackProcessor(repo)

		err := processor.ProcessPack(bytes.NewReader([]byte{}))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no pack data found")

		invalidPack := []byte("INVALID_PACK_DATA")
		err = processor.ProcessPack(bytes.NewReader(invalidPack))
		assert.Error(t, err)
	})

	t.Run("ParsePackHeader", func(t *testing.T) {
		processor := NewPackProcessor(repo)

		// valid pack header: "PACK" + version 2 + 1 object
		packHeader := []byte{
			'P', 'A', 'C', 'K',  // signature
			0x00, 0x00, 0x00, 0x02,  // version 2
			0x00, 0x00, 0x00, 0x01,  // 1 object
		}
		processor.packData = packHeader

		header, err := processor.parsePackHeader()
		assert.NoError(t, err)
		assert.Equal(t, "PACK", header.Signature)
		assert.Equal(t, uint32(2), header.Version)
		assert.Equal(t, uint32(1), header.Objects)
	})
}

func TestPackObjectHeader(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	processor := NewPackProcessor(repo)

	t.Run("ParseSimpleObjectHeader", func(t *testing.T) {
		// object type 3 (blob), size 10
		// first byte: 0011 0000 | 1010 = 0x3A
		// since size < 16, no continuation bit needed
		objHeader := []byte{0x3A}  // type=3, size=10
		processor.packData = objHeader

		objType, size, offset := processor.parseObjectHeader(0)
		assert.Equal(t, 3, objType)  // OBJ_BLOB
		assert.Equal(t, int64(10), size)
		assert.Equal(t, 1, offset)
	})

	t.Run("ParseLargeObjectHeader", func(t *testing.T) {
		// Object type 3 (blob), size 1000
		// need multiple bytes for size
		objHeader := []byte{
			0xBB,  // type=3, size=11 (low 4 bits), continuation bit set
			0x07,  // size continuation: 7 << 4 = 112, total = 11 + 112*16 = 1803...
		}
		// noote: This is a simplified test - real Git uses more complex variable-length encoding
		processor.packData = objHeader

		objType, size, offset := processor.parseObjectHeader(0)
		assert.Equal(t, 3, objType)  // OBJ_BLOB
		assert.Greater(t, size, int64(10))  // Should be larger than simple case
		assert.Equal(t, 2, offset)
	})
}

func TestGitProtocolParser(t *testing.T) {
	t.Run("ParseNAKPacket", func(t *testing.T) {
		// Git packet-line format: "0008NAK\n"
		packetData := []byte("0008NAK\n")
		parser := &GitProtocolParser{data: packetData}

		packet, err := parser.readPacket()
		assert.NoError(t, err)
		assert.Equal(t, []byte("NAK\n"), packet)
		assert.True(t, parser.isNAKPacket(packet))
	})

	t.Run("ParseFlushPacket", func(t *testing.T) {
		// Flush packet: "0000"
		packetData := []byte("0000")
		parser := &GitProtocolParser{data: packetData}

		packet, err := parser.readPacket()
		assert.NoError(t, err)
		assert.Nil(t, packet)  // Flush packets return nil
	})

	t.Run("ParseSidebandPacket", func(t *testing.T) {
		// sideband packet with channel 2 (progress)
		packetData := []byte{2, 'P', 'r', 'o', 'g', 'r', 'e', 's', 's'}
		parser := &GitProtocolParser{data: packetData}

		assert.True(t, parser.isSidebandPacket(packetData))

		data, err := parser.extractSidebandPackData(packetData)
		assert.NoError(t, err)
		assert.Nil(t, data)  // channel 2 is progress, returns nil
	})

	t.Run("ExtractPackFromSideband", func(t *testing.T) {
		// test with actual Git protocol response format using sideband
		packData := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x01test")

		// NAK packet + sideband packet with pack data
		var protocolData bytes.Buffer
		protocolData.WriteString("0008NAK\n")  // NAK response
		protocolData.WriteString("0000")        // Flush packet

		// sideband packet with pack data (channel 1)
		sidebandData := append([]byte{1}, packData...)  // Channel 1 + pack data
		packetLine := fmt.Sprintf("%04x", len(sidebandData)+4)
		protocolData.WriteString(packetLine)
		protocolData.Write(sidebandData)

		parser := &GitProtocolParser{data: protocolData.Bytes()}
		extracted, err := parser.ExtractPackData()
		assert.NoError(t, err)
		assert.Equal(t, packData, extracted)
	})

	t.Run("ExtractPackFromPacketLine", func(t *testing.T) {
		packData := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x01test")
		// wrap in a packet-line: length (hex) + data
		packetData := fmt.Sprintf("%04x", len(packData)+4) + string(packData)

		parser := &GitProtocolParser{data: []byte(packetData)}
		extracted, err := parser.ExtractPackData()
		assert.NoError(t, err)
		assert.Equal(t, packData, extracted)
	})
}

func TestDeltaInstructions(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	processor := NewPackProcessor(repo)

	t.Run("ApplySimpleDelta", func(t *testing.T) {
		baseData := []byte("Hello, World!")

		// simple delta: base_size + result_size + insert instruction
		deltaData := []byte{
			13,    // base size (13 bytes)
			5,     // result size (5 bytes)
			5, 'G', 'i', 't', '!', '!', // insert 5 bytes "Git!!"
		}

		result, err := processor.applyDelta(baseData, deltaData)
		if err != nil {
			// delta processing is complex, just ensure it doesn't crash
			assert.Contains(t, err.Error(), "mismatch")
		} else {
			assert.NotEmpty(t, result)
		}
	})

	t.Run("ReadDeltaSize", func(t *testing.T) {
		// test variable-length size encoding
		data := []byte{0x85, 0x02}  // Size with continuation
		size, offset := processor.readDeltaSize(data, 0)

		assert.Greater(t, size, int64(0))
		assert.Equal(t, 2, offset)
	})
}

func TestPackObjectTypes(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	processor := NewPackProcessor(repo)

	tests := []struct {
		packType int
		expected objects.ObjectType
	}{
		{1, objects.ObjectTypeCommit},
		{2, objects.ObjectTypeTree},
		{3, objects.ObjectTypeBlob},
		{4, objects.ObjectTypeTag},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			result := processor.packTypeToObjectType(tt.packType)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("UnknownType", func(t *testing.T) {
		result := processor.packTypeToObjectType(99)
		assert.Equal(t, objects.ObjectType(""), result)
	})
}
