package constants

//go:generate stringer -type=ConnState -linecomment -output=conn_state_stringer.go
type ConnState int64

const (
	ConnStateInit   ConnState = iota // init
	ConnStateStatus                  // status
	ConnStateLogin                   // login
	ConnStatePlay                    // play
	ConnStateClose                   // close
)
