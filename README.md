# TRemote plugin play_audio_mp3flac - Jukebox

TRemote is a service for ARM based Linux computers. It lets you remote control *things* on these kind of machines, specifically over Bluetooth. There is no limit to what you can remote control. You can access a list of predefined actions, you can execute executables and shell scripts, you can issue http request, and you can invoke your own or 3rd party native code plugins.

This repository contains the complete Go source code of a remote control plugin application. You can use this plugin as-is. You can also use it as a template to implement similar or extended functionality.

TRemote plugin **play_audio_mp3flac** implements a Jukebox application for MP3 and FLAC content.
This is useful sample code, demonstrating how things can be implemented in the 
context of a TRemote plugin. This is also a very useful application 
that works reliably and is fun to use.

play_audio_mp3flac makes use of the following projects: [bobertlo/go-mpg123](http://github.com/bobertlo/go-mpg123), [mewkiz/flac](http://github.com/mewkiz/flac), [gordonklaus/portaudio](http://github.com/gordonklaus/portaudio). These packages will be automatically fetched by "go build".

# Building the plugin

TRemote plugins are based on Go Modules. You need to use [Go v1.11](https://dl.google.com/go/go1.11.linux-armv6l.tar.gz) (direct dl link for linux-armv6l) to build this plugin. Before you start make sure your "go version" command returns "go version go1.11 linux/arm".

After cloning this repository enter the following command to build the plugin:

```
CGO_ENABLED=1 go build -buildmode=plugin play_audio_mp3flac.go
```
This will create the "play_audio_mp3flac.so" binary. Copy the binary over to your Tremote folder, add a mapping entry like the one shown below to your mapping.txt file and restart the TRemote service. You can now invoke your plugin functionality via a Bluetooh remote control.

If you intend to modify this plugin or create a similar one, you should also take a look at the [TRemote plugin base package](https://github.com/mehrvarz/tremote_plugin).

You can use the runtrp tool to run TRemote plugins from the command line. Download runtrp for [linux-armv6l](https://github.com/mehrvarz/tremote_plugin/tree/master/bin.linux.ARM6) and for [linux-amd64](https://github.com/mehrvarz/tremote_plugin/tree/master/bin.linux.AMD64).


# Button mapping

The following entries in "mapping.txt" bind the jukebox to several buttons and hand over different local locations:

```
P3, Jazz,  play_audio|/media/sda1/Music/Jazz
P4, Pop,   play_audio|/media/sda1/Music/Pop
P8, Rock,  play_audio|/media/sda1/Music/Rock
```

A short button press will start a random playback of audio files from a specified folder. 
From this moment forward audio playback will continue randomly until it will be stopped. 
When the same button is pressed again, audio playback will skip to the next song. 
A history of played songs is kept. If the same button is long-pressed (at least 500ms), 
audio playback will skip back one song. Long press again will skip back 
another song. The play history is also being used to prevent songs from 
playing again in short order. A different button can optionally be used 
to implement a pause function.

Note that a plugin does not know anything about remote controls, about Bluetooth or how a button event is delivered to it. It only cares about the implementation of the response action. The mapping file bindes the two sides together.



