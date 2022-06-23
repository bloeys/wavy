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

type SoundType int

const (
	SoundType_Unknown SoundType = iota
	SoundType_MP3
)

type SampleRate int

const (
	SampleRate_44100 SampleRate = 44100
	SampleRate_48000 SampleRate = 48000
)

type SoundChannelCount int

const (
	SoundChannelCount_1 SoundChannelCount = 1
	SoundChannelCount_2 SoundChannelCount = 2
)

type SoundBitDepth int

const (
	SoundBitDepth_1 SoundBitDepth = 1
	SoundBitDepth_2 SoundBitDepth = 2
)

type SoundMode int

const (
	SoundMode_Streaming SoundMode = iota
	SoundMode_Memory
)

var (
	Ctx          *oto.Context
	SamplingRate SampleRate
	ChanCount    SoundChannelCount
	BitDepth     SoundBitDepth

	//Pre-defined errors
	ErrunknownSoundType = errors.New("unknown sound type. Sound file extensions must be: .mp3")
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

	//Bytes is an io.ReadSeeker over an open file or over a buffer containing the uncompressed sound file.
	//Becomes nil after close
	Bytes io.ReadSeeker

	Info SoundInfo
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
	currBytePos, _ = s.Bytes.Seek(0, io.SeekCurrent)
	currBytePos -= int64(s.Player.UnplayedBufferSize())

	lenInMS := float64(s.Info.Size-currBytePos) / float64(SamplingRate) / 4 * 1000
	return time.Duration(lenInMS) * time.Millisecond
}

func (s *Sound) IsClosed() bool {
	return s.Bytes == nil
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

	s.Bytes = nil
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
		s.Bytes = dec
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

		s.Bytes = dec
		s.Info.Size = dec.Length()
		s.Player = Ctx.NewPlayer(dec)
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
