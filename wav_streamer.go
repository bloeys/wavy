package wavy

import (
	"io"
	"os"
	"sync"

	"github.com/go-audio/wav"
)

var _ io.ReadSeeker = &WavStreamer{}

type WavStreamer struct {
	F        *os.File
	Dec      *wav.Decoder
	Pos      int64
	PCMStart int64

	//TODO: This is currently needed because of https://github.com/hajimehoshi/oto/issues/171
	//We should be able to delete once its resolved
	mutex sync.Mutex
}

func (ws *WavStreamer) Read(outBuf []byte) (bytesRead int, err error) {

	ws.mutex.Lock()
	bytesRead, err = ws.Dec.PCMChunk.Read(outBuf)
	ws.Pos += int64(bytesRead)
	ws.mutex.Unlock()

	return bytesRead, err
}

func (ws *WavStreamer) Seek(offset int64, whence int) (int64, error) {

	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	n, err := ws.Dec.Seek(offset, whence)
	if err != nil {
		return n, err
	}

	//The underlying parser can not seek back, so in that case we have to rewind the decoder
	//then seek forward.
	if n < ws.Pos {

		err = ws.Dec.Rewind()
		if err != nil {
			return 0, err
		}

		//Anything before PCMStart is not valid sound, so the minimum seek back we allow is till PCMStart.
		if n >= ws.PCMStart {

			n, err = ws.Dec.Seek(offset, whence)
			if err != nil {
				return n, err
			}
		} else {
			n = ws.PCMStart
		}
	}

	ws.Pos = n
	return n, err
}

func (ws *WavStreamer) Size() int64 {
	return ws.Dec.PCMLen()
}

func NewWavStreamer(f *os.File, wavDec *wav.Decoder) (*WavStreamer, error) {

	err := wavDec.FwdToPCM()
	if err != nil {
		return nil, err
	}

	currPos, err := wavDec.Seek(0, 1)
	if err != nil {
		return nil, err
	}

	return &WavStreamer{
		F:        f,
		Dec:      wavDec,
		Pos:      currPos,
		PCMStart: currPos,
		mutex:    sync.Mutex{},
	}, nil
}
