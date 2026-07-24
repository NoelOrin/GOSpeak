package srs

import "GOSpeak/internal/sfu"

var _ sfu.Provider = (*Service)(nil)
var _ sfu.StreamProvider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)
