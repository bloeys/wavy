package wavy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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
	ErrunknownSoundType = errors.New("unknown sound type. Sound file extensions must be: .mp3")
)

//SoundInfo contains static info about a loaded sound file
type SoundInfo struct {
	Type SoundType
	Mode SoundMode

	SamplingRate SampleRate
	ChanCount    SoundChannelCount
	BitDepth     SoundBitDepth

	//Size is the sound's size in bytes
	Size int64
}

type Sound struct {
	//Becomes nil after close
	Ctx    *oto.Context
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
	//Should never run, but just in case TotalTimeMS was a bit inaccurate
	for s.Player.IsPlaying() {
	}
}

//TotalTime returns the time taken to play the entire sound.
//Safe to use after close
func (s *Sound) TotalTime() time.Duration {
	//Number of bytes divided by sampling rate (which is bytes consumed per second), then divide by 4 because each sample is 4 bytes in go-mp3
	lenInMS := float64(s.Info.Size) / float64(s.Info.SamplingRate) / 4 * 1000
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

	lenInMS := float64(s.Info.Size-currBytePos) / float64(s.Info.SamplingRate) / 4 * 1000
	return time.Duration(lenInMS) * time.Millisecond
}

func (s *Sound) IsClosed() bool {
	return s.Ctx == nil
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

	s.Ctx = nil
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
func NewSoundStreaming(fpath string, sr SampleRate, chanCount SoundChannelCount, bitDepth SoundBitDepth) (s *Sound, err error) {

	//Error checking filetype
	soundType := SoundType_Unknown
	if strings.HasSuffix(fpath, ".mp3") {
		soundType = SoundType_MP3
	}

	if soundType == SoundType_Unknown {
		return nil, ErrunknownSoundType
	}

	//Preparing oto context
	otoCtx, readyChan, err := oto.NewContext(int(sr), int(chanCount), int(bitDepth))
	if err != nil {
		return nil, err
	}
	<-readyChan

	//We read file but don't close so the player can stream the file any time later
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}

	s = &Sound{
		Ctx:      otoCtx,
		FileDesc: file,
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Streaming,

			SamplingRate: sr,
			ChanCount:    chanCount,
			BitDepth:     bitDepth,
		},
	}

	//Load file depending on type
	if soundType == SoundType_MP3 {

		dec, err := mp3.NewDecoder(file)
		if err != nil {
			return nil, err
		}

		s.Info.Size = dec.Length()
		s.Player = otoCtx.NewPlayer(dec)
		s.Bytes = dec
	}

	return s, nil
}

//NewSoundMem loads the entire sound file into memory and plays from that
func NewSoundMem(fpath string, sr SampleRate, chanCount SoundChannelCount, bitDepth SoundBitDepth) (s *Sound, err error) {

	//Error checking filetype
	soundType := SoundType_Unknown
	if strings.HasSuffix(fpath, ".mp3") {
		soundType = SoundType_MP3
	}

	if soundType == SoundType_Unknown {
		return nil, ErrunknownSoundType
	}

	//Preparing oto context
	otoCtx, readyChan, err := oto.NewContext(int(sr), int(chanCount), int(bitDepth))
	if err != nil {
		return nil, err
	}
	<-readyChan

	fileBytes, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	bytesReader := bytes.NewReader(fileBytes)
	s = &Sound{
		Ctx: otoCtx,
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Memory,

			SamplingRate: sr,
			ChanCount:    chanCount,
			BitDepth:     bitDepth,
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
		s.Player = otoCtx.NewPlayer(dec)
	}

	return s, nil
}
