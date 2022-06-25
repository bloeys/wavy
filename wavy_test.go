package wavy_test

import (
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

	//Test repeat playing
	s2 := wavy.CopyInMemSound(s)
	s2.SetVolume(0.25)

	s.PlaySync()  //Already finished, should not play
	s2.PlaySync() //Should play from beginning

	//Test seek and play
	s2.SeekToPercent(0.2)
	s2.PlaySync()

	s2.SeekToTime(400 * time.Millisecond)
	s2.PlaySync()
}

func TestByteCountFromPlayTime(t *testing.T) {

	got := wavy.ByteCountFromPlayTime(400 * time.Millisecond)
	expected := int64(70560)
	if got != expected {
		t.Errorf("Expected '%d' but got '%d'\n", expected, got)
		return
	}
}

func TestPlayTimeFromByteCount(t *testing.T) {

	got := wavy.PlayTimeFromByteCount(70560)
	expected := 400 * time.Millisecond
	if got != expected {
		t.Errorf("Expected '%d' but got '%d'\n", expected, got)
		return
	}
}
