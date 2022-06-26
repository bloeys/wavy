package wavy

type SoundType int

const (
	SoundType_Unknown SoundType = iota
	SoundType_MP3
	SoundType_WAV
	SoundType_OGG
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
