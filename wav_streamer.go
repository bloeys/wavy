package wavy

import (
	"io"
	"os"

	"github.com/go-audio/wav"
)

var _ io.ReadSeeker = &WavStreamer{}

type WavStreamer struct {
	F        *os.File
	Dec      *wav.Decoder
	Pos      int64
	PCMStart int64
}

func (ws *WavStreamer) Read(outBuf []byte) (bytesRead int, err error) {

	bytesRead, err = ws.Dec.PCMChunk.Read(outBuf)
	ws.Pos += int64(bytesRead)

	return bytesRead, err
}

func (ws *WavStreamer) Seek(offset int64, whence int) (int64, error) {

	// This will only seek the underlying file but not the actual decoder because it can't seek
	n, err := ws.Dec.Seek(offset, whence)
	if err != nil {
		return n, err
	}

	// Since underlying decoder can't seek back, if the requested movement is back we have to rewind the decoder
	// then seek forward to the requested position.
	if n < ws.Pos {

		err = ws.Dec.Rewind()
		if err != nil {
			return 0, err
		}

		// Anything before PCMStart is not valid sound, so the minimum seek back we allow is PCMStart
		if n < ws.PCMStart {
			n = ws.PCMStart
		} else {
			n, err = ws.Dec.Seek(offset, whence)
			if err != nil {
				return n, err
			}
		}
	}

	ws.Pos = n
	return n, err
}

// Size returns number of bytes
func (ws *WavStreamer) Size() int64 {
	return ws.Dec.PCMLen()
}

func NewWavStreamer(f *os.File, wavDec *wav.Decoder) (*WavStreamer, error) {

	err := wavDec.FwdToPCM()
	if err != nil {
		return nil, err
	}

	// The actual data starts somewhat within the file, not at 0
	currPos, err := wavDec.Seek(0, 1)
	if err != nil {
		return nil, err
	}

	return &WavStreamer{
		F:        f,
		Dec:      wavDec,
		Pos:      currPos,
		PCMStart: currPos,
	}, nil
}
