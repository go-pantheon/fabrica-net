package codec

import (
	"encoding/binary"
	"io"

	"github.com/go-pantheon/fabrica-net/internal/bufreader"
	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/errors"
)

func Encode(w io.Writer, pack xnet.Pack) error {
	if err := binary.Write(w, binary.BigEndian, xnet.PackLenSize+uint32(len(pack))); err != nil {
		return errors.Wrap(err, "write pack len failed")
	}

	if _, err := w.Write(pack); err != nil {
		return errors.Wrapf(err, "write pack failed")
	}

	return nil
}

func Decode(r io.Reader) (xnet.Pack, error) {
	var totalLen uint32

	err := binary.Read(r, binary.BigEndian, &totalLen)
	if err != nil {
		return nil, err
	}

	if totalLen < xnet.PackLenSize || totalLen > xnet.MaxPackSize {
		return nil, errors.Errorf("packet len=%d must greater than %d and less than %d", totalLen, xnet.PackLenSize, xnet.MaxPackSize)
	}

	packLen := totalLen - xnet.PackLenSize

	buf := make([]byte, packLen)

	n, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	if n != int(packLen) {
		return nil, errors.New("short read")
	}

	return xnet.Pack(buf), nil
}

func BufDecode(r *bufreader.Reader) (xnet.Pack, error) {
	totalLen, err := r.ReadUint32()
	if err != nil {
		return nil, errors.Wrapf(err, "read pack len failed")
	}

	if totalLen < xnet.PackLenSize || totalLen > xnet.MaxPackSize {
		return nil, errors.Errorf("packet len=%d must greater than %d and less than %d", totalLen, xnet.PackLenSize, xnet.MaxPackSize)
	}

	buf, err := r.ReadFull(int(totalLen - xnet.PackLenSize))
	if err != nil {
		return nil, errors.Wrapf(err, "read pack body failed. len=%d", totalLen)
	}

	return xnet.Pack(buf), nil
}
