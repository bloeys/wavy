package wavy_test

import (
	"io"
	"math"
	"testing"
	"time"

	"github.com/bloeys/wavy"
)

const (
	sampleRate      = 44100
	bitDepthInBytes = 2
	channelNum      = 2
)

//This is from the Oto example
type SineWave struct {
	freq   float64
	length int64
	pos    int64

	remaining []byte
}

//Implements io.Read interface for SineWave
func (s *SineWave) Read(buf []byte) (int, error) {
	if len(s.remaining) > 0 {
		n := copy(buf, s.remaining)
		copy(s.remaining, s.remaining[n:])
		s.remaining = s.remaining[:len(s.remaining)-n]
		return n, nil
	}

	if s.pos == s.length {
		return 0, io.EOF
	}

	eof := false
	if s.pos+int64(len(buf)) > s.length {
		buf = buf[:s.length-s.pos]
		eof = true
	}

	var origBuf []byte
	if len(buf)%4 > 0 {
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	length := float64(sampleRate) / float64(s.freq)

	num := (bitDepthInBytes) * (channelNum)
	p := s.pos / int64(num)
	switch bitDepthInBytes {
	case 1:
		for i := 0; i < len(buf)/num; i++ {
			const max = 127
			b := int(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max)
			for ch := 0; ch < channelNum; ch++ {
				buf[num*i+ch] = byte(b + 128)
			}
			p++
		}
	case 2:
		for i := 0; i < len(buf)/num; i++ {
			const max = 32767
			b := int16(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max)
			for ch := 0; ch < channelNum; ch++ {
				buf[num*i+2*ch] = byte(b)
				buf[num*i+1+2*ch] = byte(b >> 8)
			}
			p++
		}
	}

	s.pos += int64(len(buf))

	n := len(buf)
	if origBuf != nil {
		n = copy(origBuf, buf)
		s.remaining = buf[n:]
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}

func NewSineWave(freq float64, duration time.Duration) *SineWave {
	l := channelNum * bitDepthInBytes * sampleRate * int64(duration) / int64(time.Second)
	l = l / 4 * 4
	return &SineWave{
		freq:   freq,
		length: l,
	}
}

func TestSound(t *testing.T) {

	fatihaFilepath := "./test_audio_files/Fatiha.mp3"
	tadaFilepath := "./test_audio_files/tada.mp3"
	const fatihaLenMS = 55484

	//Streaming
	s, err := wavy.NewSoundStreaming(fatihaFilepath, wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
	if err != nil {
		t.Errorf("Failed to load streaming sound with path '%s'. Err: %s\n", fatihaFilepath, err)
		return
	}

	s.PlayAsync()
	time.Sleep(1 * time.Second)

	remTime := s.RemainingTime()
	if remTime.Milliseconds() >= fatihaLenMS-900 {
		t.Errorf("Expected time to be < %dms but got %dms in streaming sound\n", fatihaLenMS-900, remTime.Milliseconds())
		return
	}

	if err := s.Close(); err != nil {
		t.Errorf("Closing streaming sound failed. Err: %s\n", err)
		return
	}

	totalTime := s.TotalTime()
	if totalTime.Milliseconds() != fatihaLenMS {
		t.Errorf("Expected time to be %dms but got %dms in streaming sound\n", fatihaLenMS, totalTime.Milliseconds())
		return
	}

	//In-Memory
	s, err = wavy.NewSoundMem(fatihaFilepath, wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", fatihaFilepath, err)
		return
	}

	s.PlayAsync()
	time.Sleep(1 * time.Second)

	remTime = s.RemainingTime()
	if remTime.Milliseconds() >= fatihaLenMS-900 {
		t.Errorf("Expected time to be < %dms but got %dms in memory sound\n", fatihaLenMS-900, remTime.Milliseconds())
		return
	}

	if err := s.Close(); err != nil {
		t.Errorf("Closing in-memory sound failed. Err: %s\n", err)
		return
	}

	totalTime = s.TotalTime()
	if totalTime.Milliseconds() != fatihaLenMS {
		t.Errorf("Expected time to be %dms but got %dms in memory sound\n", fatihaLenMS, totalTime.Milliseconds())
		return
	}

	//Memory 'tada.mp3'
	s, err = wavy.NewSoundMem(tadaFilepath, wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", tadaFilepath, err)
		return
	}

}
