# wavy

Wavy is a high-level, easy to use, and cross-platform Go sound library built on top of <https://github.com/hajimehoshi/oto>.

Wavy supports both streaming sounds from disk and playing from memory.

- [wavy](#wavy)
  - [Supported Platforms](#supported-platforms)
  - [Supported audio formats](#supported-audio-formats)
  - [Usage](#usage)
    - [Installation](#installation)
    - [Basics](#basics)
    - [Controls](#controls)

## Supported Platforms

Supported platforms are:

- Windows
- macOS
- Linux
- FreeBSD
- OpenBSD
- Android
- iOS
- WebAssembly

If you are using iOS or Linux please check [this](https://github.com/hajimehoshi/oto#prerequisite).

## Supported audio formats

- MP3
- Wav/Wave

## Usage

### Installation

First install Wavy with `go get github.com/bloeys/wavy`, and if you are using iOS or Linux check [this](https://github.com/hajimehoshi/oto#prerequisite).

### Basics

You can start playing sounds with a few lines:

```go
import (
    "github.com/bloeys/wavy"
)

func main() {

    //At the start if your program you should init wavy and tell it the sampling rate of your sounds (usually 44100),
    //the number of channels (usually 2) and the number of bytes per channel (usually 2).
    //
    //These settings will be used for all sounds regardless of their actual settings
    err := wavy.Init(wavy.SampleRate_44100, wavy.SoundChannelCount_2, wavy.SoundBitDepth_2)
    if err != nil {
        panic("Failed to init wavy. Err: " + err.Error())
    }

    //Here we load a sound into memory
    mySound, err := wavy.NewSoundMem("./my-sound.mp3")
    if err != nil {
        panic("Failed to create new sound. Err: " + err.Error())
    }

    //Now we set volume of this sound to 50% then play the sound
    //and wait for it to finish (PlayAsync plays in the background)
    mySound.SetVolume(0.5)
    mySound.PlaySync()

    //Since the sound finished playing, lets reset to start
    //by seeking to 0% then play again. Seeking to 0.5 then playing will start from the middle the sound.
    mySound.SeekToPercent(0)
    mySound.PlayAsync()

    //The sound is playing the background, so lets wait for it to finish
    mySound.Wait()
}
```

If you are dealing with large sound files you might want to stream from a file (play as you go), this will only
use a small amount of memory, but is less flexible and might be slower to seek.

Here is an example streaming a sound:

```go

    //Here we load a sound into memory
    mySound, err := wavy.NewSoundStreaming("./my-sound.mp3")
    if err != nil {
        panic("Failed to create new sound. Err: " + err.Error())
    }

    //Rest is the same...
```

### Controls

Once you have loaded a sound you can:

- Pause/Resume
- Set volume per sound
- Play synchronously or asynchronously
- Loop a number of times or infinitely
- Check total play time and remaining time
- Seek to any position (by percent or time) of the sound even when its already playing
- Wait for a sound to finish playing once
- Wait for a looping sound to finish all its repeats
- (only in-memory) Load it once but have many versions play from different positions simultaneously (e.g. one gun starting to shoot, another ending its shot sound)
- (only in-memory) Take a short clip from a sound (e.g. keep only the first half of the sound)

Code examples of everything:

```go

//Load wav into memory
mySound, err := wavy.NewSoundMem("./my-sound.wav")
if err != nil {
    panic("Failed to create new sound. Err: " + err.Error())
}

//Play for ~1s then Pause
mySound.PlayAsync()
time.sleep(1 * time.Second)
mySound.Pause()

//Resume and play till end
mySound.PlaySync()

//Set volume to 25%
mySound.SetVolume(0.25)

//Play the sound three times and wait for all 3 plays to finish. Negative numbers will play infinitely till paused
mySound.LoopAsync(3)
mySound.WaitLoop()

//Check playtime
println("Time to play full sound:", mySound.TotalTime().Seconds())
println("Time remaining till sound finishes:", mySound.RemainingTime().Seconds())

//Play sound from the middle
mySound.SeekToPercent(0.5)
mySound.PlaySync()

//Play sound from time=5s
mySound.SeekToTime(5 * time.Second)
mySound.PlaySync()

//Wait for sound to finish if started async
mySound.Wait()

//Start looping infinitely then stop
mySound.LoopAsync(-1)
time.Sleep(1 * time.Second)
mySound.Pause()

//
// Things only possible for in-memory sounds
//

//1. Playing sound many times simultaneously without loading it multiple times

//We reuse the underlying sound data but get two independent sounds with their own controls!
//This operation is fast so you can do it a lot
mySound2 := CopyInMemSound(mySound)

//Set one to play from the beginning and the other to play from the middle
mySound.SeekToPercent(0)
mySound2.SeekToPercent(0.5)

//Play both simultaneously
mySound.PlayAsync()
mySound2.PlayAsync()

//Wait for both to finish
mySound.Wait()
mySound2.Wait()

//2. Cut parts of a sound

//Here we get a new sound that only has the first half of the sound.
//This operation is very quick and does not duplicate the underlying data
clippedSound := ClipInMemSoundPercent(mySound, 0, 0.5)
clippedSound.PlaySync()
```

Aside from per sound controls, there are a few global controls:

```go
wavy.PauseAllSounds()
wavy.ResumeAllSounds()
```
