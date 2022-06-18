package wavy_test

import (
	"io"
	"math"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto/v2"
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

func TestWavy(t *testing.T) {

	const freqToUse = 523.3

	c, ready, err := oto.NewContext(44100, 2, 2)
	if err != nil {
		t.Errorf("Failed to create oto context. Err: %e\n", err)
		return
	}
	<-ready

	playDuration := 1 * time.Second
	player := c.NewPlayer(NewSineWave(freqToUse, playDuration))
	player.SetVolume(0.75)
	player.Play()

	time.Sleep(playDuration)
	runtime.KeepAlive(player)
}

func TestMP3(t *testing.T) {

	audioFPath := "./test_audio_files/Fatiha.mp3"
	f, err := os.Open(audioFPath)
	if err != nil {
		t.Errorf("Failed to open '%s'. Err: %s\n", audioFPath, err)
		return
	}
	defer f.Close()

	dec, err := mp3.NewDecoder(f)
	if err != nil {
		t.Errorf("Failed to decode mp3 file. Err: %s\n", err)
		return
	}

	c, ready, err := oto.NewContext(dec.SampleRate(), 2, 2)
	if err != nil {
		t.Errorf("Failed to create oto context. Err: %s\n", err)
		return
	}
	<-ready

	player := c.NewPlayer(dec)
	player.SetVolume(0.75)
	player.Play()

	time.Sleep(1 * time.Second)

	//This is to ensure GC doesn't collect player/context. Without it no sound might play, or plays for very small amount of time.
	runtime.KeepAlive(player)
}
