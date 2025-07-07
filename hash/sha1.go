package hash

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

func ComputeSHA1(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func ComputeObjectHash(objType string, data []byte) string {
	// Git object format: "type size\0content"
	header := fmt.Sprintf("%s %d\x00", objType, len(data))
	content := append([]byte(header), data...)
	return ComputeSHA1(content)
}

func ValidateHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}

	for _, char := range hash {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}

	return true
}

func ShortHash(hash string, length int) string {
	if length <= 0 || length > len(hash) {
		return hash
	}
	return hash[:length]
}
