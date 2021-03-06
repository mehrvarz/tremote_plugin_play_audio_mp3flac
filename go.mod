module github.com/mehrvarz/tremotehost-ix/play_audio_mp3flac

require (
	github.com/bobertlo/go-mpg123 v0.0.0-20140824082338-d2238336e6db
	github.com/dhowden/tag v0.0.0-20180815181651-82440840077f
	github.com/gordonklaus/portaudio v0.0.0-20180817120803-00e7307ccd93
	github.com/mehrvarz/go_queue v1.0.1
	github.com/mehrvarz/log v1.0.1
	github.com/mehrvarz/tremote_plugin v1.0.13
	github.com/mewkiz/flac v1.0.5
)

//replace github.com/mehrvarz/tremote_plugin => ../tremote_plugin
//replace github.com/mehrvarz/go_queue => ../../tremote-packages/go_queue
