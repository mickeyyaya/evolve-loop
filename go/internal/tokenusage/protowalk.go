package tokenusage

const (
	wireVarint = 0
	wireI64    = 1
	wireBytes  = 2
	wireI32    = 5
)

func readVarint(b []byte, i int) (uint64, int, error) {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if i >= len(b) {
			return 0, i, errMalformedProto
		}
		c := b[i]
		i++
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v, i, nil
		}
	}
	return 0, i, errMalformedProto
}

type protoField struct {
	num, wire, varint uint64
	bytes             []byte
}

func nextField(b []byte, i int) (protoField, int, error) {
	key, i, err := readVarint(b, i)
	if err != nil {
		return protoField{}, i, err
	}
	f := protoField{num: key >> 3, wire: key & 7}
	switch f.wire {
	case wireVarint:
		f.varint, i, err = readVarint(b, i)
	case wireBytes:
		var n uint64
		n, i, err = readVarint(b, i)
		if err == nil && uint64(len(b)-i) < n {
			err = errMalformedProto
		}
		if err == nil {
			f.bytes, i = b[i:i+int(n)], i+int(n)
		}
	case wireI64:
		i, err = skip(b, i, 8)
	case wireI32:
		i, err = skip(b, i, 4)
	default:
		err = errMalformedProto
	}
	return f, i, err
}

func skip(b []byte, i, n int) (int, error) {
	if len(b)-i < n {
		return i, errMalformedProto
	}
	return i + n, nil
}

func walkFields(b []byte, each func(protoField) error) error {
	for i := 0; i < len(b); {
		f, next, err := nextField(b, i)
		if err != nil {
			return err
		}
		if err := each(f); err != nil {
			return err
		}
		i = next
	}
	return nil
}
