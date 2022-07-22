package wavy

import (
	"errors"
	"io"
)

// Pre-defined errors
var (
	ErrInvalidWhence   = errors.New("invalid whence value. Must be: io.SeekStart, io.SeekCurrent, or io.SeekEnd")
	ErrNegativeSeekPos = errors.New("negative seeker position")
)

var _ io.ReadSeeker = &SoundBuffer{}

type SoundBuffer struct {
	Data []byte

	// Pos is the starting position of the next read
	Pos int64
}

// Read only returns io.EOF when bytesRead==0 and no more input is available
func (sb *SoundBuffer) Read(outBuf []byte) (bytesRead int, err error) {

	bytesRead = copy(outBuf, sb.Data[sb.Pos:])
	if bytesRead == 0 {
		return 0, io.EOF
	}

	sb.Pos += int64(bytesRead)
	return bytesRead, nil
}

// Seek returns the new position.
// An error is only returned if the whence is invalid or if the resulting position is negative.
//
// If the resulting position is >=len(SoundBuffer.Data) then future Read() calls will return io.EOF
func (sb *SoundBuffer) Seek(offset int64, whence int) (int64, error) {

	newPos := sb.Pos
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos += offset
	case io.SeekEnd:
		newPos = int64(len(sb.Data)) + offset
	default:
		return 0, ErrInvalidWhence
	}

	if newPos < 0 {
		return 0, ErrNegativeSeekPos
	}

	sb.Pos = newPos
	return sb.Pos, nil
}

// Copy returns a new SoundBuffer that uses the same `Data` but with an independent ReadSeeker.
// This allows you to have many readers all reading from different positions of the same buffer.
//
// The new buffer will have its starting position set to io.SeekStart (`Pos=0`)
func (sb *SoundBuffer) Copy() *SoundBuffer {
	return &SoundBuffer{
		Data: sb.Data,
		Pos:  0,
	}
}
