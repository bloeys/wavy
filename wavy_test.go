package wavy_test

import (
	"io"
	"testing"
	"time"

	"github.com/bloeys/wavy"
)

func TestSound(t *testing.T) {

	fatihaFilepath := "./test_audio_files/Fatiha.mp3"
	tadaFilepath := "./test_audio_files/tada.mp3"
	const fatihaLenMS = 55484

	err := wavy.Init(wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
	if err != nil {
		t.Errorf("Failed to init wavy. Err: %s\n", err)
		return
	}

	//Streaming
	s, err := wavy.NewSoundStreaming(fatihaFilepath)
	if err != nil {
		t.Errorf("Failed to load streaming sound with path '%s'. Err: %s\n", fatihaFilepath, err)
		return
	}

	s.PlayAsync()
	time.Sleep(1 * time.Second)
	s.Player.Pause()

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
	s, err = wavy.NewSoundMem(fatihaFilepath)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", fatihaFilepath, err)
		return
	}

	s.PlayAsync()
	time.Sleep(1 * time.Second)
	s.Player.Pause()

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
	s, err = wavy.NewSoundMem(tadaFilepath)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", tadaFilepath, err)
		return
	}
	s.PlaySync()

	s.Player.Reset()
	s.Bytes.Seek(0, io.SeekStart)
	s.PlaySync()
}
