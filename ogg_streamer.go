package wavy

import (
	"io"
	"os"

	"github.com/jfreymuth/oggvorbis"
)

var _ io.ReadSeeker = &OggStreamer{}

type OggStreamer struct {
	F   *os.File
	Dec *oggvorbis.Reader
}

func (ws *OggStreamer) Read(outBuf []byte) (floatsRead int, err error) {

	readerBuf := make([]float32, len(outBuf)/2)
	floatsRead, err = ws.Dec.Read(readerBuf)
	F32ToUnsignedPCM16(readerBuf[:floatsRead], outBuf)

	return floatsRead * 2, err
}

func (ws *OggStreamer) Seek(offset int64, whence int) (int64, error) {

	//This is because ogg expects position in samples not bytes
	offset /= BytesPerSample

	switch whence {
	case io.SeekStart:
		if err := ws.Dec.SetPosition(offset); err != nil {
			return 0, err
		}

	case io.SeekCurrent:

		if err := ws.Dec.SetPosition(ws.Dec.Position() + offset); err != nil {
			return 0, err
		}

	case io.SeekEnd:
		if err := ws.Dec.SetPosition(ws.Dec.Length() + offset); err != nil {
			return 0, err
		}
	}

	return ws.Dec.Position() * BytesPerSample, nil
}

//Size returns number of bytes
func (ws *OggStreamer) Size() int64 {
	return ws.Dec.Length() * BytesPerSample
}

func NewOggStreamer(f *os.File, dec *oggvorbis.Reader) *OggStreamer {
	return &OggStreamer{
		F:   f,
		Dec: dec,
	}
}
