package wavy

import (
	"bytes"
	"errors"
	"fmt"
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

var (
	ErrunknownSoundType = errors.New("unknown sound type. Sound file extensions must be: .mp3")
)

type Sound struct {
	Ctx    *oto.Context
	Player oto.Player
	Type   SoundType

	//FileDesc is the file descriptor of the sound file being streamed. This is only set if NewSoundStreaming is used
	FileDesc *os.File

	//BytesReader is a reader from a buffer containing the entire sound file
	BytesReader *bytes.Reader
}

func (s *Sound) PlayAsync() {
	s.Player.Play()
}

func (s *Sound) PlaySync() {

	s.Player.Play()
	for s.Player.IsPlaying() {
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Sound) Close() error {

	var fdErr error = nil
	if s.FileDesc != nil {
		fdErr = s.FileDesc.Close()
	}

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

	//Load file depending on type
	s = &Sound{Ctx: otoCtx, Type: soundType, FileDesc: file}
	if soundType == SoundType_MP3 {

		dec, err := mp3.NewDecoder(file)
		if err != nil {
			return nil, err
		}

		s.Player = otoCtx.NewPlayer(dec)
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

	//Load file depending on type
	s = &Sound{Ctx: otoCtx, Type: soundType, BytesReader: bytesReader}
	if soundType == SoundType_MP3 {

		dec, err := mp3.NewDecoder(bytesReader)
		if err != nil {
			return nil, err
		}

		s.Player = otoCtx.NewPlayer(dec)
	}

	return s, nil
}
