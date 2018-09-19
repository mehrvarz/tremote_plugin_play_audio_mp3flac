/* 
TRemote plugin play_audio_mp3flac implements a jukebox for m3p and flac content.
This is useful sample code, demonstrating how things can be implemented in the 
context of a TRemote plugin. This is also a very useful standalone implementation 
of a jukebox, that is reliable and fun to use.
play_audio_mp3flac is bound to a single button. A short press starts random 
playback of audio files from a specified folder. From this moment forward audio
playback will continue randomly until it is stopped externally. When the same 
button is pressed again, audio playback will skip to the next song. If the same 
button is long-pressed (at least 500ms), audio playback will skip back one song. 
play_audio_mp3flac keeps a history of 50 songs. Long press again will skip back 
one song at a time. The play history is also being used to prevent songs from 
randomly playing again in short order. A different button can optionally be used 
to implement a pause function.
*/
package main

import (
	"fmt"
	"time"
	"strings"
	"math/rand"
	"os"
	"io"
	"io/ioutil"
	"sync"
	"runtime"

	"github.com/gordonklaus/portaudio"
	"github.com/dhowden/tag"
	"github.com/bobertlo/go-mpg123/mpg123"
	"github.com/mewkiz/flac"

	"github.com/mehrvarz/tremote_plugin"
	"github.com/mehrvarz/go_queue"
	"github.com/mehrvarz/log"
)

const (
	queueSize           = 50
)

var (
	pluginname          = "play_audio_mp3flac"
	logm                log.Logger
	songsPlayedQueueMap map[string]*go_queue.Queue
	instanceNumber      = 0
	abortFolderShuffle  = false
	waitingForOlderInstanceToStop = false
)

func init() {
	songsPlayedQueueMap = make(map[string]*go_queue.Queue)	// a map of queues, one per folder
}

/*
Action() is the entry point for any TRemote plugin. We need to make sure 
Action() will always return quickly. This is why we start new goroutines for 
opertations that take more time. The first thing we want to do is to figure 
out if we are coping with a short or a long press event.
*/
func Action(log log.Logger, pid int, longpress bool, pressedDuration int64, homedir string, rcs *tremote_plugin.RemoteControlSpec, ph tremote_plugin.PluginHelper, wg *sync.WaitGroup) error {
	var lock_Mutex	sync.Mutex
	lock_Mutex.Lock()
	logm = log

	if instanceNumber==0 {
		firstinstance(homedir)
	}
	instanceNumber++

	strArray := rcs.StrArray
	if longpress {
		strArray = rcs.StrArraylong
	}

	// here we try to find out if this is a shortpress or longpress button event
	if !longpress && pressedDuration==0 {
		// button has just been pressed; is still pressed
		go func() {
			// let's see if it becomes a longpress
			time.Sleep(tremote_plugin.LongPressDelay * time.Millisecond)
			if (*ph.PLastPressedMS)[pid]>0 && !(*ph.PLastPressActionDone)[pid] {
				// button is still pressed; job not yet taken care of; this is a longpress; let's take care of it
				(*ph.PLastPressActionDone)[pid] = true
				actioncall(true, pid, strArray, ph, wg)
			}
		}()

	} else {
		// button has been released
		if (*ph.PLastPressActionDone)[pid] {
			// button event has already been taken care of
		} else {
			// we need to take care of it here
			(*ph.PLastPressActionDone)[pid] = true
			//logm.Debugf("%s short press pid=%d %d",pluginname,pid,(*ph.PLastPressActionDone)[pid])
			go func() {
				actioncall(false, pid, strArray, ph, wg)
			}()
		}
	}

	lock_Mutex.Unlock()
	return nil
}

func firstinstance(homedir string) {
	// do things here that are only supposed to execute on first call
	//readConfig(homedir)
}

/*
actioncall() is always called with a new goroutine, separate from the framework.
Some of the libraries we use may panic; we catch this here in order to not bring 
the framework down.
Our next task is to stop any other music player that might be currently active.
If longpress is set true, we skip back one song using songsPlayedQueue.Pop().
If longpress is set to false, we start our main jukebox funktion and enter a 
random song playback loop.
*/
func actioncall(longpress bool, pid int, strArray []string, ph tremote_plugin.PluginHelper, wg *sync.WaitGroup) {
	var lock_Mutex	sync.Mutex
	lock_Mutex.Lock()

	wg.Add(1)
	defer func() {
		if err := recover(); err != nil {
			wg.Done()
 			logm.Errorf("%s panic=%s", pluginname, err)
			buf := make([]byte, 1<<16)
			runtime.Stack(buf, true)
 			logm.Errorf("%s stack=\n%s", pluginname, buf)
		}
	}()

	instance := instanceNumber

	// set PIdLastPressed (only music playing plugins need to do this)
	*ph.PIdLastPressed = pid
	logm.Infof("%s (%d) actioncall longpress=%v arg=%s", pluginname, instance, longpress, strArray[0])

	if waitingForOlderInstanceToStop {
		// an older instance of this plugin is already waiting for an even older instance to stop (!)
		// we likely have too many overlapping actioncall() instances: giving up on this new instance
		logm.Warningf("%s (%d) exit on waitingForOlderInstanceToStop",pluginname,instance)
		lock_Mutex.Unlock()
		return
	}

	if *ph.StopAudioPlayerChan!=nil {
		waitingForOlderInstanceToStop = true
		logm.Debugf("%s (%d) stopping other instance...",pluginname,instance)
		*ph.StopAudioPlayerChan <- true
		time.Sleep(200 * time.Millisecond)
	} else {
		// No instance of our player is currently active. There may be some other audio playing instance.
		// Stop whatever audio player may currently be active.
		waitingForOlderInstanceToStop = true
		logm.Debugf("%s (%d) on start no audio Plugin active -> StopCurrentAudioPlayback()",pluginname,instance)
		ph.StopCurrentAudioPlayback()
		time.Sleep(200 * time.Millisecond)
	}

	var ourStopAudioPlayerChan chan bool = nil
	if *ph.StopAudioPlayerChan==nil {
		// this allows parent to stop us
		ourStopAudioPlayerChan = make(chan bool)
		*ph.StopAudioPlayerChan = ourStopAudioPlayerChan
	} else {
		// should never happen
		logm.Warningf("%s (%d) StopAudioPlayerChan!=nil",pluginname,instance)
	}
	if *ph.PauseAudioPlayerChan==nil {
		// this allows parent to pause us
		*ph.PauseAudioPlayerChan = make(chan bool)
	} else {
		// should never happen
		logm.Warningf("%s (%d) PauseAudioPlayerChan!=nil",pluginname,instance)
	}
	waitingForOlderInstanceToStop = false
	lock_Mutex.Unlock()
	folder := strArray[0]

	songsPlayedQueue := songsPlayedQueueMap[folder]
	if songsPlayedQueue==nil {
		songsPlayedQueue = go_queue.NewQueue(queueSize)
		songsPlayedQueueMap[folder] = songsPlayedQueue
	}

	if longpress {
		// play previous song from songsPlayedQueue
		logm.Infof("%s (%d) start long-press step back",pluginname,instance)

		currentFile := songsPlayedQueue.Pop()
		if currentFile == nil {
			logm.Infof("mapping Play_audio end of queue")
			ph.PrintStatus("end of queue")
			goto end
		}
		previousFile := songsPlayedQueue.Pop()
		if previousFile == nil {
			logm.Infof("mapping Play_audio end of queue")
			ph.PrintStatus("end of queue")
			goto end
		}

		pathfile := folder + "/" + previousFile.Value
		if playSong(previousFile.Value,pathfile,ph,instance,songsPlayedQueue) {
			logm.Debugf("%s (%d) done playSong step back - manually aborted",pluginname, instance)
			goto end
		}
		if abortFolderShuffle {
			// possibly unexpected portaudioStream.Write() issue
			logm.Debugf("%s (%d) done playSong step back - abortFolderShuffle",pluginname, instance)
			goto end
		}
		logm.Debugf("%s (%d) done playSong step back",pluginname, instance)
		// continue with file loop

	} else {
		// short press
		// continue with file loop
	}

	// file loop over files in a folder
	logm.Infof("%s (%d) start folder loop",pluginname,instance)
	for {
		fileName := ""
		pathfile := ""
		fileArray, err := ioutil.ReadDir(folder) // []os.FileInfo
		if err == nil {
			logm.Debugf("%s (%d) start folder %s loop...",pluginname, instance, folder)
			// read all files from folder
			if len(fileArray)<1 {
				logm.Warningf("%s folder %s is empty",pluginname,pathfile)
				ph.PrintStatus("folder "+pathfile+" is empty")
				break
			}

			// randomize order of files in fileArray / shuffle play
			randomizeFileInfoArray(fileArray)

			// find next mp3 or flac file that has not yet been played
			i := 0
			for {
				if i>=len(fileArray) {
					logm.Infof("%s reached end of folder list",pluginname)
					break
				}
				nextFile := fileArray[i] // os.FileInfo
				if nextFile == nil {
					logm.Warningf("%s nextFile is null - skip",pluginname)
				} else if nextFile.IsDir() {
					//logm.Debugf("%s '%s' is a directory - skip",pluginname, nextFile.Name())
				} else if songsPlayedQueue != nil && songsPlayedQueue.InQueue(nextFile.Name()) {
					logm.Debugf("%s '%s' found inQueue - skip", pluginname, nextFile.Name())
				} else if strings.HasSuffix(nextFile.Name(),".flac") {
					fileName = nextFile.Name()
					logm.Debugf("%s '%s' flac", pluginname, fileName)
					break
				} else if strings.HasSuffix(nextFile.Name(),".mp3") {
					fileName = nextFile.Name()
					logm.Debugf("%s '%s' mp3", pluginname, fileName)
					break
				}
				i++
			}

			if fileName=="" {
				// if PopOldest() fails, do not continue
				if songsPlayedQueue.PopOldest()!=nil {
					logm.Infof("%s found no song; try again after removing oldes song from queue",pluginname)
					//ph.PrintStatus("cannot find any unplayed files")
					continue
				}
				logm.Infof("%s found no unplayed song; giving up",pluginname)
				ph.PrintStatus("cannot find any unplayed files - giving up")
				break
			}

			pathfile = folder+"/"+fileName
			logm.Debugf("%s pathfile=%s", pluginname, pathfile)

		} else {
			// arg is not a folder but a single file; play file; do not loop
			fileName = folder
			pathfile = fileName
			abortFolderShuffle = true
		}
		
		if playSong(fileName,pathfile,ph,instance,songsPlayedQueue) {
			logm.Debugf("%s (%d) done playSong - manually aborted",pluginname, instance)
			break
		}
		if abortFolderShuffle {
			// single file playback -or- unexpected/fatal portaudioStream related issue
			logm.Debugf("%s (%d) exit loop on abortFolderShuffle",pluginname, instance)
			break
		}
		
		// continuing with next song...
	}

end:
	var lock_Mutex2	sync.Mutex
	lock_Mutex2.Lock()
	logm.Debugf("%s (%d) exit",pluginname, instance)
	abortFolderShuffle = false
	if *ph.StopAudioPlayerChan!=nil && *ph.StopAudioPlayerChan==ourStopAudioPlayerChan {
		*ph.StopAudioPlayerChan = nil
	} else {
		// should never happen
		if *ph.StopAudioPlayerChan==nil {
			logm.Warningf("%s (%d) StopAudioPlayerChan was nil",pluginname, instance)
		} else if *ph.StopAudioPlayerChan!=ourStopAudioPlayerChan {
			logm.Warningf("%s (%d) StopAudioPlayerChan!=ourStopAudioPlayerChan",pluginname, instance)
		} else {
			logm.Warningf("%s (%d) what?",pluginname, instance)
		}
	}
	if *ph.PauseAudioPlayerChan!=nil {
		*ph.PauseAudioPlayerChan = nil
	} else {
		// should never happen
		logm.Warningf("%s (%d) ph.PauseAudioPlayerChan was nil",pluginname, instance)
	}
	wg.Done()
	lock_Mutex2.Unlock()
}

func playSong(fileName string, pathfile string, ph tremote_plugin.PluginHelper,
		instance int, songsPlayedQueue *go_queue.Queue) bool {
	// returns true if manually aborted or on fatal error
	isMp3 := false
	isFlac := false
	if strings.HasSuffix(pathfile,".flac") {
		isFlac = true
		logm.Debugf("%s (%d) playSong %s as flac",pluginname, instance, fileName)
	} else {
		// anything else: treat as mp3
		isMp3 = true
		logm.Debugf("%s (%d) playSong %s as mp3",pluginname, instance, fileName)
	}

	// read id3 tags
	id3tags := ""
	var id3_artwork* tag.Picture = nil
	r, err := os.Open(pathfile)
	if err != nil {
		logm.Warningf("%s open file %s err=%s",pluginname, pathfile, err.Error())
		ph.PrintStatus("error open file %s"+pathfile)
		return false
	}
	defer r.Close()

	m, err := tag.ReadFrom(r)
	if err != nil {
		logm.Warningf("%s read tags err=%s", pluginname, err.Error())
	} else {
		// remove control and extended characters from title, artist, album
		title  := stripCtlAndExtFromUTF8(m.Title())
		artist := stripCtlAndExtFromUTF8(m.Artist())
		album  := stripCtlAndExtFromUTF8(m.Album())
		logm.Debugf("%s (%d) tags: [%s, %s, %s]", pluginname, instance, title, artist, album)

		id3tags = title
		if artist!="" {
			if id3tags=="" {
				id3tags = artist
			} else {
				id3tags = id3tags + " - " + artist
			}
		}
		if album!="" {
			if id3tags=="" {
				id3tags = album
			} else {
				id3tags = id3tags + " - " + album
			}
		}
		if id3tags == "" {
			id3tags = fileName
		}
		logm.Infof("%s tag string: [%s]", pluginname, id3tags)
		
		id3_artwork = m.Picture()
		if id3_artwork==nil {
			logm.Infof("%s tag artwork: none", pluginname)
		} else {
			logm.Infof("%s tag artwork: Ext=[%s] MIME=[%s] Type=[%s] size=%d", pluginname, 
				id3_artwork.Ext, id3_artwork.MIMEType, id3_artwork.Type, len(id3_artwork.Data))
		}
	}

	songsPlayedQueue.Push(&go_queue.Node{fileName})
	logm.Debugf("%s (%d) start player thread...", pluginname,instance)

	var sampleRate int64
	var channels int
	var bitsPerSample int
	var bytesPerSample int
	var mp3decoder *mpg123.Decoder
	var flacstream *flac.Stream
	var framesPerBuffer int		// determines the size of the decode buffer (mp3-only; flac sets this itself)
	var outbufElements int		// number of outbuf elements, based on size of decoded block

	if isMp3 {
		// create mpg123 mp3decoder instance
		mp3decoder, err = mpg123.NewDecoder("")
		if err != nil {
			logm.Warningf("%s error creating mp3 decoder err=%s",pluginname, err.Error())
			ph.PrintStatus("error creating mp3 decoder "+err.Error())
			return false
		}

		err = mp3decoder.Open(pathfile)
		if err != nil {
			logm.Warningf("%s error open mp3 file err=%s",pluginname, err.Error())
			ph.PrintStatus("error open mp3 file "+err.Error())
			return false
		}
		defer mp3decoder.Close()

		// get audio format information
		sampleRate, channels, _ = mp3decoder.GetFormat()
		bitsPerSample  = 16
		bytesPerSample = bitsPerSample/8
		logm.Infof("%s mpg123 sampleRate=%d channels=%d", pluginname, sampleRate, channels)

		if sampleRate>44100 || bitsPerSample>16 {
			info := fmt.Sprintf("%d %d",bitsPerSample,sampleRate)
			ph.PrintStatus(info)
		}

		// make sure output format does not change
		mp3decoder.FormatNone()
		mp3decoder.Format(sampleRate, channels, mpg123.ENC_SIGNED_16)

	} else if isFlac {
		// create flac decoder instance
		flacstream, err = flac.Open(pathfile)
		if err != nil {
			logm.Warningf("%s error open flac file err=%s",pluginname, err.Error())
			ph.PrintStatus("error open flac file %s"+err.Error())
			return false
		}
		defer flacstream.Close()

		channels       = int(flacstream.Info.NChannels)
		sampleRate     = int64(flacstream.Info.SampleRate)
		bitsPerSample  = int(flacstream.Info.BitsPerSample)
		bytesPerSample = bitsPerSample/8
		logm.Infof("%s flac sampleRate=%d channels=%d bps=%d Bps=%d", 
			pluginname, sampleRate, channels, bitsPerSample, bytesPerSample)

		if sampleRate>44100 || bitsPerSample>16 {
			info := fmt.Sprintf("%d %d",bitsPerSample,sampleRate)
			ph.PrintStatus(info)
		}
	}

	// send id3 tags
	ph.PrintInfo(id3tags)

	// send artwork
	if id3_artwork!=nil && len(id3_artwork.Data)>0 {
		ph.ImageInfo(id3_artwork.Data,id3_artwork.MIMEType)
	} else {
		ph.ImageInfo(nil,"")
	}

	logm.Debugf("%s (%d) portaudio.Initialize()", pluginname,instance)
	portaudio.Initialize()
	defer portaudio.Terminate()

	// pump audio out
	logm.Debugf("%s (%d) pump audio out...", pluginname,instance)
	var playbackPaused = false
	var framecount     = 0
	var portaudioStream *portaudio.Stream
	var outbuf16 []int16 = nil
	var outbuf32 []int32 = nil
	var quitPlayback = false
	for {
		if playbackPaused {
			//logm.Debugf("%s playbackPaused", pluginname)
			time.Sleep(500 * time.Millisecond)

		} else {
			var j = 0
			if isMp3 {
				// copy mp3 data -> audio -> portaudio out in chunks of 16KB
				framesPerBuffer = 4096 * channels	// samples per buffer for two channels
				audioBuf := make([]byte, framesPerBuffer * bytesPerSample)
				var count int
				count, err = mp3decoder.Read(audioBuf)
				if err == mpg123.EOF {
					err = nil
					break
				}
				if err != nil {
					logm.Warningf("%s error reading audio source err=%s",pluginname, err.Error())
					ph.PrintStatus("error reading audio source")
					// skip to next song
					break
				}

				if outbuf16 == nil {
					outbufElements = count //* channels
					logm.Debugf("%s audioBuf len=%dbytes read count=%dbytes", pluginname, len(audioBuf), count)

					outbuf16 = make([]int16, framesPerBuffer)				// always assuming 16 bit from mp3
					logm.Debugf("%s framesPerBuffer=%d len(outbuf16)=%delements", 
						pluginname, framesPerBuffer, len(outbuf16))
					portaudioStream, err = 
						portaudio.OpenDefaultStream(0, channels, float64(sampleRate), framesPerBuffer, &outbuf16)
					if err != nil {
						// "Invalid sample rate"
						logm.Warningf("%s error open audio sink for playback err=%s",pluginname, err.Error())
						ph.PrintStatus("error open audio sink for playback: "+err.Error())
						abortFolderShuffle = true
						break
					}
					defer portaudioStream.Close()

					err = portaudioStream.Start()
					if err != nil {
						logm.Warningf("%s error starting audio playback err=%s",pluginname, err.Error())
						ph.PrintStatus("error starting audio playback: "+err.Error())
						abortFolderShuffle = true
						break
					}
					defer portaudioStream.Stop()

					//audioVolumeUnmute(instance)
					ph.HostCmd("AudioVolumeUnmute","")
				}

				j = 0
				for i := 0; i < count; i+=2 {
					if j<framesPerBuffer {
						outbuf16[j] = int16(audioBuf[i+1])<<8 | int16(audioBuf[i])
						j++
					}
				}

			} else {
				// flac
				frame, err := flacstream.ParseNext()
				if err != nil {
					// "invalid sync-code"
					if err == io.EOF {
						logm.Debugf("%s EOF",pluginname)
						break
					}

					logm.Warningf("%s error flacstream.ParseNext() err=%s",pluginname, err.Error())
					if strings.Index(err.Error(),"invalid sync-code")>=0 {
						//logm.Debug("%s continue on 'invalid sync-code'",pluginname)
					} else {
						break
					}
				}

				if (bytesPerSample==2 && outbuf16==nil) || (bytesPerSample==3 && outbuf32==nil) {
					outbufElements = int(frame.BlockSize) * channels
					if bytesPerSample==2 {
						outbuf16 = make([]int16, outbufElements)
						logm.Debugf("%s frame.BlockSize=%d outbufElements=%d len(outbuf16)=%d channels=%d",
							pluginname, frame.BlockSize, outbufElements, len(outbuf16), channels)
						portaudioStream, err = 
							portaudio.OpenDefaultStream(0, channels, float64(sampleRate), outbufElements, &outbuf16)
					} else if bytesPerSample==3 {
						outbuf32 = make([]int32, outbufElements)
						logm.Debugf("%s frame.BlockSize=%d outbufElements=%d len(outbuf32)=%d channels=%d",
							pluginname, frame.BlockSize, outbufElements, len(outbuf32), channels)
						// NOTE: for some reason I need outbufElements+100 on AMD64
						portaudioStream, err = 
							portaudio.OpenDefaultStream(0, channels, float64(sampleRate), outbufElements+100, &outbuf32)
					}

					if err != nil {
						logm.Warningf("%s error open audio sink for playback err=%s",pluginname, err.Error())
						ph.PrintStatus("error open audio sink for playback: "+err.Error())
						abortFolderShuffle = true
						break
					}
					defer portaudioStream.Close()

					err = portaudioStream.Start()
					if err != nil {
						logm.Warningf("%s error starting audio playback err=%s",pluginname, err.Error())
						ph.PrintStatus("error starting audio playback: "+err.Error())
						portaudioStream.Close()
						abortFolderShuffle = true
						break
					}
					defer portaudioStream.Stop()

					//audioVolumeUnmute(instance)
					ph.HostCmd("AudioVolumeUnmute","")
				}

				//logm.Debugf("%s frame.BlockSize=%d %d",pluginname, frame.BlockSize,len(frame.Subframes))
				if len(frame.Subframes) < channels {
					logm.Warningf("%s len(frame.Subframes)=%d",pluginname, len(frame.Subframes))
				} else {
					if len(frame.Subframes[0].Samples) < int(frame.BlockSize) {
						logm.Warningf("%s incomplete frame.Subframes[0].Samples < frame.BlockSize",pluginname)
					} else if len(frame.Subframes[1].Samples) < int(frame.BlockSize) {
						logm.Warningf("%s incomplete frame.Subframes[1].Samples < frame.BlockSize",pluginname)
					} else {
						j = 0
						if bytesPerSample==2 {
							for i:=0; i < int(frame.BlockSize); i++ {
								if j<outbufElements {
									outbuf16[j] = int16(frame.Subframes[0].Samples[i]); j++
									if j<outbufElements {
										outbuf16[j] = int16(frame.Subframes[1].Samples[i]); j++
									}
								}
							}
						} else {
							for i:=0; i < int(frame.BlockSize); i++ {
								if j<outbufElements {
									outbuf32[j]=frame.Subframes[0].Samples[i]<<8; j++
									if j<outbufElements {
										outbuf32[j]=frame.Subframes[1].Samples[i]<<8; j++
									}
								}
							}
						}

						//logm.Debugf("forward outbufElements=%d/%d frame.BlockSize=%d",outbufElements,j,frame.BlockSize)
					}
				}
			}

			framecount++
			//logm.Debugf("%s j=%d framecount=%d",pluginname, j,framecount)
			err = portaudioStream.Write()
			if err != nil {
				logm.Warningf("%s error writing audio data err=%s",pluginname, err.Error())
				// do not abort playback on "Output underflowed"
				if err.Error()!="Output underflowed" {
					abortFolderShuffle = true
					ph.PrintStatus("error writing audio data: "+err.Error())
					break
				}
			}
		}

		select {
		case <-*ph.StopAudioPlayerChan:
			// we are being aborted by Stop_current_stream()
			logm.Debugf("%s (%d) stopped by StopAudioPlayerChan",pluginname, instance)
			abortFolderShuffle = true
			quitPlayback = true
		case <-*ph.PauseAudioPlayerChan:
			playbackPaused = !playbackPaused
			logm.Debugf("%s (%d) pausemode set to %v",pluginname, instance, playbackPaused)
		default:
			// default is needed so that the other cases don't block
		}

		if quitPlayback {
			logm.Debugf("%s (%d) quitPlayback",pluginname, instance)
			break
		}
	}

	logm.Debugf("%s (%d) singleSongPlayback finished (framecount=%d)",pluginname, instance,framecount)
	ph.PrintInfo("")	// note: in case of inloop error, this may clear out the error-msg
	return quitPlayback
}

// for removing control and extended characters from id3tags (see: The Wanton Song - Led Zep)
// https://rosettacode.org/wiki/Strip_control_codes_and_extended_characters_from_a_string#Go
func stripCtlAndExtFromUTF8(str string) string {
	return strings.Map(func(r rune) rune {
		if r >= 32 && r < 127 {
			return r
		}
		return -1
	}, str)
}

func randomizeFileInfoArray(fileArray []os.FileInfo) {
	entries := len(fileArray)
	//logm.Infof("mapping randomizeListStrings: entries %d",entries)
	for i := 0; i < len(fileArray); i++ {
		swp1 := i //rand.Intn(entries)

		// swp2 may not be == swp1
		var swp2 int
		for {
			swp2 = rand.Intn(entries)
			if swp2 != swp1 {
				break
			}
		}

		swpEntry := fileArray[swp1]
		fileArray[swp1] = fileArray[swp2]
		fileArray[swp2] = swpEntry
	}
}

/*
func readConfig(path string) int {
	pathfile := "config.txt"
	if len(path)>0 { pathfile = path + "/config.txt" }

	file, err := os.Open(pathfile)
	if err != nil {
		logm.Debugf("readConfig from "+pathfile+" failed: %s", err.Error())
		return 0 // not fatal, we can do without config.txt
	}
	defer file.Close()

	logm.Debugf("readConfig from %s", pathfile)
	linecount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		pound := strings.Index(line, "#")
		if pound >= 0 {
			//logm.Infof("readConfig found # at pos %d",pound)
			line = line[:pound]
		}
		if line != "" {
			line = strings.TrimSpace(line)
		}
		if line != "" {
			//logm.Infof("readConfig line: ["+line+"]")
			linetokens := strings.SplitN(line, "=",2)
			//logm.Infof("readConfig tokens: [%v]",linetokens)
			if len(linetokens) >= 2 {
				key := strings.TrimSpace(linetokens[0])
				value := strings.TrimSpace(linetokens[1])
				//logm.Debugf("readConfig key=[%s] val=[%s]", key, value)
				linecount++

				switch key {
				case "audiocontrol":
					logm.Debugf("readConfig key=[%s] val=[%s]", key, value)
					AudioControl = value
				}
			}
		}
	}
	return linecount
}
*/

