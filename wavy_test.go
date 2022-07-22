package wavy_test

import (
	"testing"
	"time"

	"github.com/bloeys/wavy"
)

func TestWavy(t *testing.T) {
	t.Run("Init", InitSubtest)
	t.Run("MP3", MP3Subtest)
	t.Run("Wav", WavSubtest)
	t.Run("Ogg", OggSubtest)
}

func InitSubtest(t *testing.T) {

	err := wavy.Init(wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
	if err != nil {
		t.Errorf("Failed to init wavy. Err: %s\n", err)
		return
	}
}

func MP3Subtest(t *testing.T) {

	const fatihaFilepath = "./test_audio_files/Fatiha.mp3"
	const tadaFilepath = "./test_audio_files/tada.mp3"
	const fatihaLenMS = 55484

	// Mp3 streaming
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

	// Mp3 in-memory
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

	// 'tada.mp3' memory
	s, err = wavy.NewSoundMem(tadaFilepath)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", tadaFilepath, err)
		return
	}
	s.PlaySync()

	// Test repeat playing
	s2 := wavy.CopyInMemSound(s)
	s2.SetVolume(0.25)

	// Already finished, should not play
	s.PlaySync()

	// Should play from beginning
	s2.PlaySync()

	// Test seek and play
	s2.SeekToPercent(0.2)
	s2.PlaySync()

	s2.SeekToTime(400 * time.Millisecond)
	s2.PlaySync()

	s3 := wavy.ClipInMemSoundPercent(s2, 0, 0.25)
	s3.LoopAsync(3)
	s3.WaitLoop()
}

func WavSubtest(t *testing.T) {

	const wavFPath = "./test_audio_files/camera.wav"

	s, err := wavy.NewSoundMem(wavFPath)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", wavFPath, err)
		return
	}
	s.PlaySync()

	// Wav streaming
	s, err = wavy.NewSoundStreaming(wavFPath)
	if err != nil {
		t.Errorf("Failed to load streaming sound with path '%s'. Err: %s\n", wavFPath, err)
		return
	}
	s.PlaySync()
	s.SeekToPercent(0.5)
	s.PlaySync()
}

func OggSubtest(t *testing.T) {

	const oggFPath = "./test_audio_files/camera.ogg"
	s, err := wavy.NewSoundMem(oggFPath)
	if err != nil {
		t.Errorf("Failed to load memory sound with path '%s'. Err: %s\n", oggFPath, err)
		return
	}
	s.PlaySync()

	// Ogg streaming
	s, err = wavy.NewSoundStreaming(oggFPath)
	if err != nil {
		t.Errorf("Failed to load streaming sound with path '%s'. Err: %s\n", oggFPath, err)
		return
	}
	s.PlaySync()
	s.SeekToPercent(.5)
	s.PlaySync()
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
