package wavy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto/v2"
)

//SoundInfo contains static info about a loaded sound file
type SoundInfo struct {
	Type SoundType
	Mode SoundMode

	//Size is the sound's size in bytes
	Size int64
}

type Sound struct {
	Player oto.Player

	//FileDesc is the file descriptor of the sound file being streamed.
	//This is only set if sound is streamed, and is kept to ensure GC doesn't hit it
	FileDesc *os.File

	//Data is an io.ReadSeeker over an open file or over a buffer containing the uncompressed sound file.
	//Becomes nil after close
	Data io.ReadSeeker

	Info SoundInfo
}

//Those values are set after Init
var (
	Ctx          *oto.Context
	SamplingRate SampleRate
	ChanCount    SoundChannelCount
	BitDepth     SoundBitDepth
)

//Pre-defined errors
var (
	ErrunknownSoundType = errors.New("unknown sound type. Sound file extension must be one of: .mp3")
)

//Init prepares the default audio device and does any required setup.
//It must be called before loading any sounds
func Init(sr SampleRate, chanCount SoundChannelCount, bitDepth SoundBitDepth) error {

	otoCtx, readyChan, err := oto.NewContext(int(sr), int(chanCount), int(bitDepth))
	if err != nil {
		return err
	}
	<-readyChan

	Ctx = otoCtx
	SamplingRate = sr
	ChanCount = chanCount
	BitDepth = bitDepth

	return nil
}

//PlayAsync plays the sound in the background and returns
func (s *Sound) PlayAsync() {
	s.Player.Play()
}

//PlaySync plays the sound (if its not already playing) and waits for it to finish before returning.
func (s *Sound) PlaySync() {

	if !s.Player.IsPlaying() {
		s.Player.Play()
	}

	time.Sleep(s.RemainingTime())
	for s.Player.IsPlaying() || s.Player.UnplayedBufferSize() > 0 {
		time.Sleep(time.Millisecond)
	}
}

//TotalTime returns the time taken to play the entire sound.
//Safe to use after close
func (s *Sound) TotalTime() time.Duration {
	//Number of bytes divided by sampling rate (which is bytes consumed per second), then divide by 4 because each sample is 4 bytes in go-mp3
	lenInMS := float64(s.Info.Size) / float64(SamplingRate) / 4 * 1000
	return time.Duration(lenInMS) * time.Millisecond
}

//RemainingTime returns the time left in the clip, which is affected by pausing/resetting/seeking of the sound.
//Returns zero after close
func (s *Sound) RemainingTime() time.Duration {

	if s.IsClosed() {
		return 0
	}

	var currBytePos int64
	currBytePos, _ = s.Data.Seek(0, io.SeekCurrent)
	currBytePos -= int64(s.Player.UnplayedBufferSize())

	lenInMS := float64(s.Info.Size-currBytePos) / float64(SamplingRate) / 4 * 1000
	return time.Duration(lenInMS) * time.Millisecond
}

func (s *Sound) IsClosed() bool {
	return s.Data == nil
}

//Close will clean underlying resources, and the 'Ctx' and 'Bytes' fields will be made nil.
//Repeated calls are no-ops
func (s *Sound) Close() error {

	if s.IsClosed() {
		return nil
	}

	var fdErr error = nil
	if s.FileDesc != nil {
		fdErr = s.FileDesc.Close()
	}

	s.Data = nil
	playerErr := s.Player.Close()

	if playerErr == nil && fdErr == nil {
		return nil
	}

	if playerErr != nil && fdErr != nil {
		return fmt.Errorf("closingFileErr: %s; underlyingPlayerErr: %s", fdErr.Error(), playerErr.Error())
	}

	if playerErr != nil {
		return playerErr
	}

	return fdErr
}

//NewSoundStreaming plays sound by streaming from a file, so no need to load the entire file into memory.
func NewSoundStreaming(fpath string) (s *Sound, err error) {

	//Error checking filetype
	soundType := SoundType_Unknown
	if strings.HasSuffix(fpath, ".mp3") {
		soundType = SoundType_MP3
	}

	if soundType == SoundType_Unknown {
		return nil, ErrunknownSoundType
	}

	//We read file but don't close so the player can stream the file any time later
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}

	s = &Sound{
		FileDesc: file,
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Streaming,
		},
	}

	//Load file depending on type
	if soundType == SoundType_MP3 {

		dec, err := mp3.NewDecoder(file)
		if err != nil {
			return nil, err
		}

		s.Info.Size = dec.Length()
		s.Player = Ctx.NewPlayer(dec)
		s.Data = dec
	}

	return s, nil
}

//NewSoundMem loads the entire sound file into memory and plays from that
func NewSoundMem(fpath string) (s *Sound, err error) {

	//Error checking filetype
	soundType := SoundType_Unknown
	if strings.HasSuffix(fpath, ".mp3") {
		soundType = SoundType_MP3
	}

	if soundType == SoundType_Unknown {
		return nil, ErrunknownSoundType
	}

	fileBytes, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	bytesReader := bytes.NewReader(fileBytes)
	s = &Sound{
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Memory,
		},
	}

	//Load file depending on type
	if soundType == SoundType_MP3 {

		dec, err := mp3.NewDecoder(bytesReader)
		if err != nil {
			return nil, err
		}

		finalBuf, err := ReadAllFromReader(dec, 0, uint64(dec.Length()))
		if err != nil {
			return nil, err
		}

		sb := &SoundBuffer{Data: finalBuf}
		s.Data = sb
		s.Player = Ctx.NewPlayer(sb)
		s.Info.Size = int64(len(sb.Data))
	}

	return s, nil
}

func GetSoundFileType(fpath string) SoundType {

	ext := path.Ext(fpath)
	switch ext {
	case "mp3":
		return SoundType_MP3
	default:
		return SoundType_Unknown
	}
}

//ReadAllFromReader takes an io.Reader and reads until error or io.EOF.
//
//If io.EOF is reached then read bytes are returned with a nil error.
//If the reader returns an error that's not io.EOF then everything read till that point is returned along with the error
//
//readingBufSize is the buffer used to read from reader.Read(). Bigger values might read more efficiently.
//If readingBufSize<4096 then readingBufSize is set to 4096
//
//ouputBufSize is used to set the capacity of the final buffer to be returned. This can greatly improve performance
//if you know the size of the output. It is allowed to have an outputBufSize that's smaller or larger than what the reader
//ends up returning
func ReadAllFromReader(reader io.Reader, readingBufSize, ouputBufSize uint64) ([]byte, error) {

	if readingBufSize < 4096 {
		readingBufSize = 4096
	}

	tempBuf := make([]byte, readingBufSize)
	finalBuf := make([]byte, 0, ouputBufSize)
	for {

		readBytesCount, err := reader.Read(tempBuf)
		finalBuf = append(finalBuf, tempBuf[:readBytesCount]...)

		if err != nil {
			if err == io.EOF {
				return finalBuf, nil
			}
			return finalBuf, err
		}
	}
}
