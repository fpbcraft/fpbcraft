package inventory

import "io"

const curseForgeMurmurMultiplier uint32 = 0x5bd1e995

func isCurseForgeWhitespace(b byte) bool {
	return b == 9 || b == 10 || b == 13 || b == 32
}

type normalizedLengthWriter struct {
	n uint32
}

func (writer *normalizedLengthWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		if !isCurseForgeWhitespace(b) {
			writer.n++
		}
	}
	return len(data), nil
}

func computeCurseForgeFingerprint(reader io.Reader, normalizedLength uint32) (uint32, error) {
	hash := uint32(1) ^ normalizedLength
	var tail [4]byte
	tailLen := 0
	buffer := make([]byte, 64*1024)

	for {
		n, err := reader.Read(buffer)
		for _, b := range buffer[:n] {
			if isCurseForgeWhitespace(b) {
				continue
			}
			tail[tailLen] = b
			tailLen++
			if tailLen == 4 {
				word := uint32(tail[0]) | uint32(tail[1])<<8 | uint32(tail[2])<<16 | uint32(tail[3])<<24
				word *= curseForgeMurmurMultiplier
				word ^= word >> 24
				word *= curseForgeMurmurMultiplier
				hash *= curseForgeMurmurMultiplier
				hash ^= word
				tailLen = 0
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}

	switch tailLen {
	case 3:
		hash ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		hash ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		hash ^= uint32(tail[0])
		hash *= curseForgeMurmurMultiplier
	}

	hash ^= hash >> 13
	hash *= curseForgeMurmurMultiplier
	hash ^= hash >> 15
	return hash, nil
}
