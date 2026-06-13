package imagetool

import "fmt"

func gifIsAnimated(data []byte) (bool, error) {
	if len(data) < 13 {
		return false, fmt.Errorf("invalid GIF: header is too short")
	}
	pos := 13
	packed := data[10]
	if packed&0b10000000 != 0 {
		pos += 3 * (1 << ((packed & 0b00000111) + 1))
	}
	frames := 0
	for pos < len(data) {
		marker := data[pos]
		pos++
		switch marker {
		case 0x3B:
			return frames > 1, nil
		case 0x21:
			if pos >= len(data) {
				return false, fmt.Errorf("invalid GIF: truncated extension")
			}
			pos++
			next, err := skipGIFSubblocks(data, pos)
			if err != nil {
				return false, err
			}
			pos = next
		case 0x2C:
			frames++
			if frames > 1 {
				return true, nil
			}
			next, err := skipGIFImage(data, pos)
			if err != nil {
				return false, err
			}
			pos = next
		default:
			return false, fmt.Errorf("invalid GIF: unexpected block marker")
		}
	}
	return false, fmt.Errorf("invalid GIF: missing trailer")
}

func skipGIFImage(data []byte, pos int) (int, error) {
	if pos+9 > len(data) {
		return pos, fmt.Errorf("invalid GIF: truncated image descriptor")
	}
	packed := data[pos+8]
	pos += 9
	if packed&0b10000000 != 0 {
		pos += 3 * (1 << ((packed & 0b00000111) + 1))
	}
	if pos >= len(data) {
		return pos, fmt.Errorf("invalid GIF: missing image data")
	}
	pos++
	return skipGIFSubblocks(data, pos)
}

func skipGIFSubblocks(data []byte, pos int) (int, error) {
	for pos < len(data) {
		size := int(data[pos])
		pos++
		if size == 0 {
			return pos, nil
		}
		pos += size
		if pos > len(data) {
			return pos, fmt.Errorf("invalid GIF: truncated sub-block")
		}
	}
	return pos, fmt.Errorf("invalid GIF: missing sub-block terminator")
}
