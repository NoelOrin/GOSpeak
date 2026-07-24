package model

import "testing"

func TestNormalizeRoomType(t *testing.T) {
	if NormalizeRoomType("text") != RoomTypeText {
		t.Fatal("text")
	}
	if NormalizeRoomType("") != RoomTypeVoice {
		t.Fatal("empty -> voice")
	}
	if NormalizeRoomType("weird") != RoomTypeVoice {
		t.Fatal("weird -> voice")
	}
}
