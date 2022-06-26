package wavy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"time"

	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto/v2"
	"github.com/jfreymuth/oggvorbis"
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
	Info   SoundInfo

	//File is the file descriptor of the sound file being streamed.
	//This is only set if sound is streamed, and is kept to ensure GC doesn't hit it
	File *os.File

	//Data is an io.ReadSeeker over an open file or over a buffer containing the uncompressed sound file.
	//Becomes nil after close
	Data io.ReadSeeker

	IsLooping bool
}

//Those values are set after Init
var (
	Ctx *oto.Context

	SamplingRate SampleRate
	ChanCount    SoundChannelCount
	BitDepth     SoundBitDepth

	BytesPerSample int64
	BytesPerSecond int64
)

//Pre-defined errors
var (
	errUnknownSoundType = errors.New("unknown sound type. Sound file extension must be one of: .mp3")
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

	BytesPerSample = int64(chanCount) * int64(bitDepth)
	BytesPerSecond = BytesPerSample * int64(SamplingRate)

	return nil
}

//Wait blocks until sound finishes playing. If the sound is not playing Wait returns immediately.
//In the worst case (Wait sleeping then sound immediately paused), Wait will block ~4% of the total play time.
//In most other cases Wait should be accurate to ~1ms.
//
//If you want to wait for all loops to finish then use WaitLoop
func (s *Sound) Wait() {

	if !s.IsPlaying() {
		return
	}

	//We wait the remaining time in 25 chunks so that if the sound was paused since wait was called we don't keep blocking
	sleepTime := s.RemainingTime() / 25
	for s.Player.IsPlaying() {
		time.Sleep(sleepTime)
	}

	//If there is anything left it should be tiny so we check frequently
	for s.Player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
}

//WaitLoop waits until the sound is no longer looping
func (s *Sound) WaitLoop() {

	for s.IsLooping {
		s.Wait()
	}
}

//PlayAsync plays the sound in the background and returns.
func (s *Sound) PlayAsync() {
	s.Player.Play()
}

//PlaySync calls PlayAsync() followed by Wait()
func (s *Sound) PlaySync() {
	s.PlayAsync()
	s.Wait()
}

//LoopAsync plays the sound 'timesToPlay' times.
//If timesToPlay<0 then it is played indefinitely until paused
//If timesToPlay==0 then the sound is not played.
//If a sound is already playing then it will be paused then resumed in a looping manner
func (s *Sound) LoopAsync(timesToPlay int) {

	if timesToPlay == 0 {
		return
	}

	if s.IsPlaying() {
		s.Pause()

		if s.IsLooping {
			s.WaitLoop()
		} else {
			s.Wait()
		}
	}

	s.PlayAsync()
	timesToPlay--
	s.IsLooping = true
	go func() {

		if timesToPlay < 0 {

			for {

				s.Wait()

				//Check is here because we don't want to seek back if we got paused
				if !s.IsLooping {
					break
				}

				s.SeekToPercent(0)
				s.PlayAsync()
			}

		} else {

			for timesToPlay > 0 {

				timesToPlay--
				s.Wait()

				//Check is here because we don't want to seek back if we got paused
				if !s.IsLooping {
					break
				}

				s.SeekToPercent(0)
				s.PlayAsync()
			}
		}

		s.IsLooping = false
	}()
}

//TotalTime returns the time taken to play the entire sound.
//Safe to use after close
func (s *Sound) TotalTime() time.Duration {
	return PlayTimeFromByteCount(s.Info.Size)
}

//RemainingTime returns the time left in the clip, which is affected by pausing/resetting/seeking of the sound.
//Returns zero after close
func (s *Sound) RemainingTime() time.Duration {

	if s.IsClosed() {
		return 0
	}

	currBytePos, _ := s.Data.Seek(0, io.SeekCurrent)
	currBytePos -= int64(s.Player.UnplayedBufferSize())

	return PlayTimeFromByteCount(s.Info.Size - currBytePos)
}

//SetVolume must be between 0 and 1 (both inclusive). Other values will panic.
//The default volume is 1.
func (s *Sound) SetVolume(newVol float64) {

	if newVol < 0 || newVol > 1 {
		panic("sound volume can not be less than zero or bigger than one")
	}

	s.Player.SetVolume(newVol)
}

//Volume returns the current volume
func (s *Sound) Volume() float64 {
	return s.Player.Volume()
}

func (s *Sound) Pause() {
	s.IsLooping = false
	s.Player.Pause()
}

func (s *Sound) IsPlaying() bool {
	return s.Player.IsPlaying()
}

//SeekToPercent moves the current position of the sound to the given percentage of the total sound length.
//For example, if a sound is 10s long and percent=0.5 then when the sound is played it will start from 5s.
//
//This can be used while the sound is playing.
//
//percent is clamped [0,1], so passing <0 is the same as zero, and >1 is the same as 1
func (s *Sound) SeekToPercent(percent float64) {

	if !s.IsPlaying() {
		s.Player.Reset()
	}

	percent = clamp01F64(percent)
	s.Data.Seek(int64(float64(s.Info.Size)*percent), io.SeekStart)
}

//SeekToTime moves the current position of the sound to the given duration.
//For example if you use t=5*time.Second then play you will start from 5th second.
//
//This can be used while the sound is playing.
//
//t is clamped between [0, totalTime]
func (s *Sound) SeekToTime(t time.Duration) {

	if !s.IsPlaying() {
		s.Player.Reset()
	}

	byteCount := ByteCountFromPlayTime(t)
	if byteCount < 0 {
		byteCount = 0
	} else if byteCount > s.Info.Size {
		byteCount = s.Info.Size
	}

	s.Data.Seek(byteCount, io.SeekStart)
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
	if s.File != nil {
		fdErr = s.File.Close()
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

//CopyInMemSound returns a new sound object that has identitcal info and uses the same underlying data, but with independent play controls (e.g. one playing at the start while one is in the middle).
//Since the sound data is not copied this function is very fast.
//
//Panics if the sound is not in-memory
func CopyInMemSound(s *Sound) *Sound {

	if s.Info.Mode != SoundMode_Memory {
		panic("only in-memory sounds can be copied. Please use NewSoundStreaming if you want to have multiple sound objects of a streaming sound")
	}

	sb := s.Data.(*SoundBuffer).Copy()

	p := Ctx.NewPlayer(sb)
	p.SetVolume(s.Volume())

	return &Sound{
		Player: p,
		File:   nil,
		Data:   sb,
		Info:   s.Info,
	}
}

//ClipInMemSoundPercent is like CopyInMemSound but produces a sound that plays only between from and to.
//fromPercent and toPercent must be between 0 and 1
func ClipInMemSoundPercent(s *Sound, fromPercent, toPercent float64) *Sound {

	if s.Info.Mode != SoundMode_Memory {
		panic("only in-memory sounds can be used in ClipInMemSoundPercent")
	}

	fromPercent = clamp01F64(fromPercent)
	toPercent = clamp01F64(toPercent)

	sb := s.Data.(*SoundBuffer).Copy()

	start := int64(float64(len(sb.Data)) * fromPercent)
	end := int64(float64(len(sb.Data)) * toPercent)
	sb.Data = sb.Data[start:end]

	p := Ctx.NewPlayer(sb)
	p.SetVolume(s.Volume())

	return &Sound{
		Player: p,
		File:   nil,
		Data:   sb,
		Info:   s.Info,
	}
}

func PauseAllSounds() {
	Ctx.Suspend()
}

func ResumeAllSounds() {
	Ctx.Resume()
}

//NewSoundStreaming plays sound by streaming from a file, so no need to load the entire file into memory.
//Good for large sound files
func NewSoundStreaming(fpath string) (s *Sound, err error) {

	soundType := GetSoundFileType(fpath)
	if soundType == SoundType_Unknown {
		return nil, errUnknownSoundType
	}

	//We read file but don't close so the player can stream the file any time later
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}

	s = &Sound{
		File: file,
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Streaming,
		},
	}

	err = soundFromReaderSeeker(file, s)
	if err != nil {
		return nil, getLoadingErr(fpath, err)
	}

	return s, nil
}

//NewSoundMem loads the entire sound file into memory
func NewSoundMem(fpath string) (s *Sound, err error) {

	soundType := GetSoundFileType(fpath)
	if soundType == SoundType_Unknown {
		return nil, getLoadingErr(fpath, errUnknownSoundType)
	}

	fileBytes, err := os.ReadFile(fpath)
	if err != nil {
		return nil, getLoadingErr(fpath, err)
	}

	bytesReader := bytes.NewReader(fileBytes)
	s = &Sound{
		Info: SoundInfo{
			Type: soundType,
			Mode: SoundMode_Memory,
		},
	}

	err = soundFromReaderSeeker(bytesReader, s)
	if err != nil {
		return nil, getLoadingErr(fpath, err)
	}

	return s, nil
}

func getLoadingErr(fpath string, err error) error {
	return fmt.Errorf("failed to load '%s' with err '%s'", fpath, err.Error())
}

func soundFromReaderSeeker(r io.ReadSeeker, s *Sound) error {

	if s.Info.Type == SoundType_MP3 {

		dec, err := mp3.NewDecoder(r)
		if err != nil {
			return err
		}

		finalBuf, err := ReadAllFromReader(dec, 0, uint64(dec.Length()))
		if err != nil {
			return err
		}

		sb := &SoundBuffer{Data: finalBuf}
		s.Data = sb
		s.Player = Ctx.NewPlayer(sb)
		s.Info.Size = int64(len(sb.Data))
	} else if s.Info.Type == SoundType_WAV {

		wavDec := wav.NewDecoder(r)
		err := wavDec.FwdToPCM()
		if err != nil {
			return err
		}

		finalBuf, err := ReadAllFromReader(wavDec.PCMChunk, 0, uint64(wavDec.PCMSize))
		if err != nil {
			return err
		}

		sb := &SoundBuffer{Data: finalBuf}
		s.Data = sb
		s.Player = Ctx.NewPlayer(sb)
		s.Info.Size = int64(len(sb.Data))
	} else if s.Info.Type == SoundType_OGG {

		soundData, _, err := oggvorbis.ReadAll(r)
		if err != nil {
			return err
		}

		sb := &SoundBuffer{Data: F32ToUnsignedPCM16(soundData)}
		s.Data = sb
		s.Player = Ctx.NewPlayer(sb)
		s.Info.Size = int64(len(sb.Data))
	}

	if s.Data == nil {
		panic("invalid sound type. This is probably a bug!")
	}

	return nil
}

func GetSoundFileType(fpath string) SoundType {

	ext := path.Ext(fpath)
	switch ext {
	case ".mp3":
		return SoundType_MP3
	case ".wav", ".wave":
		return SoundType_WAV
	case ".ogg":
		return SoundType_OGG
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

//PlayTimeFromByteCount returns the time taken to play this many bytes
func PlayTimeFromByteCount(byteCount int64) time.Duration {
	//timeToPlayInMs = timeToPlayInSec * 1000 = byteCount / bytesPerSecond * 1000
	lenInMs := float64(byteCount) / float64(BytesPerSecond) * 1000
	return time.Duration(lenInMs) * time.Millisecond
}

//PlayTimeFromByteCount returns how many bytes are needed to produce a sound that takes t time to play
func ByteCountFromPlayTime(t time.Duration) int64 {
	return t.Milliseconds() * BytesPerSecond / 1000
}

//clampF64 [min,max]
func clamp01F64(x float64) float64 {

	if x < 0 {
		return 0
	}

	if x > 1 {
		return 1
	}

	return x
}

//F32ToUnsignedPCM16 takes PCM data stored as float32 between [-1, 1]
//and returns a byte array of uint16, where each two subsequent bytes represent one uint16.
func F32ToUnsignedPCM16(fs []float32) []byte {

	outBuf := make([]byte, len(fs)*2)
	for i := 0; i < len(fs); i++ {

		//Remap [-1,1]->[-32768, 32767], then re-interprets the int16 as a uint16.
		//With this, the negative values are mapped into the higher half of the uint16 range,
		//while positive values remain unchanged
		x := fs[i]
		var u16 uint16
		if x < 0 {
			u16 = uint16(x * -math.MinInt16)
		} else {
			u16 = uint16(x * math.MaxInt16)
		}

		baseIndex := i * 2
		outBuf[baseIndex] = byte(u16 >> 0)
		outBuf[baseIndex+1] = byte(u16 >> 8)
	}

	return outBuf
}
