package sign

// 待定功能
import (
	"fmt"
	"os"
)

type ClientBot struct {
	Name string
}

func (*ClientBot) InitBot(path string) (bool, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	fmt.Println(file)
	return true, nil
}

func (*ClientBot) LoadingLocalBot() (bool, error) {
	_, err := os.Stat("./bot")

	if os.IsNotExist(err) {
		// 如果不存在
		err = os.Mkdir("bot", 0777)
	}

	return false, err
}

func (*ClientBot) PublishAudioTrack(filePath string) {
	// file := filePath
	// it could be url
	// branch -> file:// or http://
	// videoWidth := 1920
	// videoHeight := 1080
	// track, err := lksdk.NewLocalFileTrack(file,
	//	lksdk.ReaderTrackWithFrameDuration(33*time.Millisecond),
	//	lksdk.ReaderTrackWithOnWriteComplete(func() { fmt.Println("track finished") }),
	//)
	//if err != nil {
	//	return err
	//}
	//var room any
	//if _, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{
	//	Name: file,
	//	//VideoWidth: videoWidth,
	//	//VideoHeight: videoHeight,
	//}); err != nil {
	//	return err
	//}
}

func (*ClientBot) KickParticipant() {

}

func (*ClientBot) MuteParticipantGlobal() {

}

func (*ClientBot) MuteParticipantRoom() {

}
