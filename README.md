# TRremote plugin play_audio_mp3flac - Jukebox

TRemote is a service for ARM based Linux computers. It lets you remote control *things* on these kind of machines, specifically over Bluetooth. There is no limit to what you can remote control. You can access a list of predefined actions, you can execute executables and shell scripts, you can issue http request, and you can invoke your own or 3rd party native code plugins.

This repository contains the complete Go source code of a remote control plugin application. You can use this plugin as-is. You can also use it as a template to implement similar or extended functionality.

TRemote plugin **play_audio_mp3flac** implements a Jukebox application for MP3 and FLAC content.
This is useful sample code, demonstrating how things can be implemented in the 
context of a TRemote plugin. This is also a very useful application 
that works reliably and fun to use.


# Building the plugin

TRemote plugins are based on Go Modules. You need to use [Go v1.11](https://dl.google.com/go/go1.11.linux-armv6l.tar.gz) (direct dl link for linux-armv6l) to build TRemote plugins. The "go version" command should return "go version go1.11 linux/arm".

After cloning this repository enter the following command to build the plugin:

```
CGO_ENABLED=1 go build -buildmode=plugin
```
This will create the "play_audio_mp3flac.so" binary. Copy the binary over to your Tremote folder, add a mapping entry like the one shown below to your mapping.txt file and restart the TRemote service. You can now invoke your plugin functionality via a Bluetooh remote control.


# Button mapping

The following entry in "mapping.txt" binds the jukebox plugin to a specific button (P3) and hand over the path to the audio content (/media/sda1/Music/Jazz):

```
P3, Jazz,  play_audio|/media/sda1/Music/Jazz
```





