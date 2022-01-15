package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/laushunyu/real/constants"
	"github.com/laushunyu/real/packet"
	"github.com/laushunyu/real/stream"
	"github.com/laushunyu/real/utils"
	"github.com/laushunyu/real/world/generate"
	"github.com/seebs/nbt"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

type server struct {
	addr string
	l    net.Listener

	players []*Player
}

func (s *server) JoinPlayer(p *Player) {
	s.players = append(s.players, p)
	for _, p := range s.players {
		p.SendChat(Chat{
			Text: "[刺溜]",
			Bold: true,
			Extra: []Chat{
				{
					Text: "",
					Bold: false,
				},
				{
					Text: "欢迎 ",
					Bold: false,
				},
				{
					Text: p.Meta.User,
					Bold: true,
				},
				{
					Text: " 加入游戏",
					Bold: false,
				},
			},
		})
	}
}

func NewServer(addr string) *server {
	return &server{addr: addr}
}

func (s *server) Run() error {
	l, err := net.Listen("tcp", "0.0.0.0:25565")
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		player := NewPlayer(conn)
		// reset position and look
		player.PL = PositionAndLook{
			X:     0,
			Y:     0,
			Z:     0,
			Yaw:   0,
			Pitch: 0,
		}

		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.WithField("addr", player.Meta.RemoteAddr).Error("connection break with panic: %+v", err)
				}
			}()
			defer player.Close()

			log.Infof("%s connected.", conn.RemoteAddr())

			reader := stream.NewReader(conn)
			for {
				// wait signal to exit
				select {
				case <-player.Done():
					log.Infof("%s disconnected", conn.RemoteAddr())
					return
				default:
				}

				// read packet from conn
				pktLen, err := reader.ReadVarInt()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					log.WithError(err).Fatal("failed to read pkt len")
					return
				}
				log.WithField("len", pktLen).Debug("get pkg len")

				pktID, err := reader.ReadVarInt()
				if err != nil {
					log.WithError(err).Fatal("failed to read pkt id")
				}
				log.WithField("id", pktID).Debug("get pkg id")

				dataLen := int(pktLen) - utils.UvarintLen(pktID)
				if dataLen < 0 {
					// If the size of the buffer containing the packet data and ID (as a VarInt) is smaller than
					// the threshold specified in the packet Set Compression.
					// It will be sent as uncompressed. This is done by setting the data length as 0.
					dataLen = 0
				}
				data, err := reader.ReadRaw(dataLen)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					log.WithError(err).Error("failed to read pkt data")
					return
				}

				pkt := packet.SPacket{
					State:    player.ConnState,
					Length:   pktLen,
					PacketID: pktID,
					Data:     data,
				}
				log.Infof("receive a pkt {state=%s id=%#x data=%#x data_escape=%q}", pkt.State, pkt.PacketID, pkt.Data, pkt.Data)

				reader := stream.NewReader(bytes.NewReader(pkt.Data))

				// handler pkt
				switch player.ConnState {
				case constants.ConnStateInit:
					switch pkt.PacketID {
					case 0x00:
						// Handshake
						log.Infof("start processing handshake pkt")
						protocolVersion, err := reader.ReadVarInt()
						if err != nil {
							log.Fatal(err)
						}
						serverAddress, err := reader.ReadString()
						if err != nil {
							log.Fatal(err)
						}
						serverPort, err := reader.ReadShort()
						if err != nil {
							log.Fatal(err)
						}
						nextStateV, err := reader.ReadVarInt()
						if err != nil {
							log.Fatal(err)
						}

						nextState := constants.ConnState(nextStateV)
						log.Infof("client connect to %s:%d with protocol %x, want next state to be %q",
							serverAddress, serverPort, protocolVersion, nextState)

						player.ConnState = nextState
						continue
					}
				case constants.ConnStateStatus:
					switch pkt.PacketID {
					case 0x00:
						// Request
						serverInfo := ServerInfo{
							Version: struct {
								Name     string `json:"name"`
								Protocol int    `json:"protocol"`
							}{
								Name:     "看你爹呢",
								Protocol: constants.Protocol,
							},
							Players: struct {
								Max    int                `json:"max"`
								Online int                `json:"online"`
								Sample []ServerInfoPlayer `json:"sample"`
							}{
								Max:    8,
								Online: 1,
								Sample: []ServerInfoPlayer{
									{Name: "macoo", Id: uuid.New().String()},
								}},
							Description: struct {
								Text string `json:"text"`
							}{"爷的 minecraft"},
						}

						// parse favicon
						if raw, err := ioutil.ReadFile("favicon.png"); err == nil {
							serverInfo.Favicon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
						}

						// marshal server info
						raw, _ := json.Marshal(serverInfo)

						// send request pkt
						requestPkt := packet.NewPacket(0x00)
						if err := requestPkt.WriteString(string(raw)).Error; err != nil {
							log.Fatal(err)
						}
						log.WithField("id", pktID).Infof("send pkt reply")
						player.Send(requestPkt)

						// then server will send a pkt Ping
						pingPkt := packet.NewPacket(0x01)
						rInt := rand.Int63()
						if err := pingPkt.WriteLong(uint64(rInt)).Error; err != nil {
							log.Fatal(err)
						}
						log.WithField("id", pktID).Infof("send pkt reply with rand int = %d", rInt)
						player.Send(pingPkt)
						return

					case 0x01:
						log.WithField("id", pktID).Infof("get pkt pong back with rand int = %d", binary.BigEndian.Uint64(pkt.Data))
						// later server will receive pkt Pong
					}
				case constants.ConnStateLogin:
					switch pkt.PacketID {
					case 0x00:
						// Login Start
						player.Meta.User, _ = reader.ReadString()
						log.Infof("%s login", player.Meta.User)

						player.Meta.UserID = uuid.New()

						// send Login Success
						// 0x02
						logSuccessPkt := packet.NewPacket(0x02)
						logSuccessPkt.WriteString(player.Meta.UserID.String()).WriteString(player.Meta.User)
						player.Send(logSuccessPkt)

						// change connect state to play after success login
						player.ConnState = constants.ConnStatePlay

						// do send many data to client
						// Event::LoginStart

						// Join Game
						joinGame := packet.NewPacket(0x23)
						joinGame.
							WriteInt(0).
							WriteRaw([]byte{1}).
							WriteInt(0).
							WriteRaw([]byte{0, 20}).
							WriteString("default").
							WriteRaw([]byte{1})
						player.Send(joinGame)

						// Player Abilities
						playerAbilities := packet.NewPacket(0x2c)
						playerAbilities.
							WriteRaw([]byte{2 | 4 | 8}).
							WriteFloat(float32(1) / 20). // Flying Speed
							WriteFloat(0)                // 视角场
						player.Send(playerAbilities)

						// Player Position And Look
						spawnEntity := packet.NewPacket(0x2f)
						spawnEntity.
							WriteDouble(player.PL.X).
							WriteDouble(player.PL.Y).
							WriteDouble(player.PL.Z).
							WriteFloat(player.PL.Yaw).
							WriteFloat(player.PL.Pitch).
							WriteRaw([]byte{0xff}).
							WriteVarInt(0)
						player.Send(spawnEntity)

						// player is in game, no others cover
						// use event center to refactor
						s.JoinPlayer(player)

						// Chunk Data
						// player.Send(PackChunk(int32(player.X)%16, int32(player.Z)%16, true))

						// send spawn Chunk Data
						centerX := int32(player.PL.X) / 16
						centerZ := int32(player.PL.X) / 16
						for x := -4; x < 4; x++ {
							for z := -4; z < 4; z++ {
								player.Send(generate.GetPlainChunkDataPacket(int32(x)+centerX, int32(z)+centerZ))
							}
						}

						// do keep alive
						keepAlive := packet.NewPacket(0x1F)
						keepAlive.WriteLong(uint64(time.Now().UnixNano()))
						player.Send(keepAlive)
						go func() {
							ticker := time.NewTicker(time.Second * 30)
							defer ticker.Stop()
							for {
								select {
								case <-player.Done():
									return
								case stamp := <-ticker.C:
									keepAlive := packet.NewPacket(0x1F)
									keepAlive.WriteLong(uint64(stamp.UnixNano()))
									player.Send(keepAlive)
								}

							}
						}()
						continue
					}
				case constants.ConnStatePlay:
					log.WithField("user", player.Meta.User).Infoln("start play!")

					switch pkt.PacketID {
					case 0x13:
						// Player Abilities

					case 0x0b:
						// keep alive
					case 0x0d:
						// Player Position
						// Updates the player's XYZ position on the server.

						player.PL.X, _ = reader.ReadDouble()
						player.PL.Y, _ = reader.ReadDouble()
						player.PL.Z, _ = reader.ReadDouble()
						player.PL.OnGround, _ = reader.ReadBoolean()

						continue
					case 0x0e:
						// Player Position And Look
						// A combination of Player Look and Player Position.

						player.PL.X, _ = reader.ReadDouble()
						player.PL.Y, _ = reader.ReadDouble()
						player.PL.Z, _ = reader.ReadDouble()
						player.PL.Yaw, _ = reader.ReadFloat()
						player.PL.Pitch, _ = reader.ReadFloat()
						player.PL.OnGround, _ = reader.ReadBoolean()
						continue
					case 0x0f:
						// Player Look
						// Updates the direction the player is looking in.

						player.PL.Yaw, _ = reader.ReadFloat()
						player.PL.Pitch, _ = reader.ReadFloat()
						player.PL.OnGround, _ = reader.ReadBoolean()
						continue
					case 0x09:
						// Plugin Message
						// Mods and plugins can use this to send their data.
						continue
					case 0x04:
						// Client Settings
						// Sent when the chunkWrt connects, or when settings are changed.
						continue
					case 0x00:
						// ack Player Position And Look
						teleportID, err := reader.ReadVarInt()
						if err != nil {
							log.Fatal(err)
						}
						_ = teleportID

						continue
					case 0x08:
						// client Close Window
						continue
					case 0x02:
						// ChatMessage
						// The client sends the raw input.
						input, _ := reader.ReadString()

						if len(input) == 0 {
							continue
						}

						if input[0] == '/' {
							// command
							log.WithField("player", player.Meta.User).Infof("input command: %s", input)
							command := strings.Split(input[1:], " ")
							if len(command) > 0 {
								switch command[0] {
								case "new":
									switch command[1] {
									case "player":
										// generate new player in player's position
										log.WithField("player", player.Meta.User).Infof("generating fake player")
										// Spawn Player
										spawnPlayerUUID := uuid.New()
										spawnPlayer := packet.NewPacket(0x05)
										spawnPlayer.WriteVarInt(123).
											WriteRaw(spawnPlayerUUID[:]).
											WriteDouble(player.PL.X).
											WriteDouble(player.PL.Y).
											WriteDouble(player.PL.Z).
											WriteRaw([]byte{byte(player.PL.Yaw)}).
											WriteRaw([]byte{byte(player.PL.Pitch)}).
											WriteNbt(nbt.Compound{})
										player.Send(spawnPlayer)
									}
								}
							}
							continue
						}

						chatMessage := packet.NewPacket(0x0F)
						msg := Chat{
							Text: fmt.Sprintf("[%s]", player.Meta.User),
							Bold: true,
							Extra: []Chat{
								{
									Text: input,
									Bold: false,
								},
							},
						}
						chatMessage.WriteString(msg.String()).WriteRaw([]byte{0})
						player.Send(chatMessage)
						continue
					}
				}
				log.Warnf("unknown pkt {state=%s id=%#x data=%+v}", pkt.State, pkt.PacketID, pkt.Data)
			}
		}()
	}
}

func main() {
	srv := NewServer(":65535")
	log.Fatal(srv.Run())
}

type ServerInfo struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int                `json:"max"`
		Online int                `json:"online"`
		Sample []ServerInfoPlayer `json:"sample"`
	} `json:"players"`
	Description struct {
		Text string `json:"text"`
	} `json:"description"`
	Favicon string `json:"favicon"`
}

type ServerInfoPlayer struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type Chat struct {
	Text  string `json:"text"`
	Bold  bool   `json:"bold"`
	Extra []Chat `json:"extra,omitempty"`
}

func (c Chat) String() string {
	raw, _ := json.Marshal(c)
	return string(raw)
}

type PositionAndLook struct {
	X, Y, Z    float64
	Yaw, Pitch float32
	OnGround   bool
}

type Player struct {
	PL        PositionAndLook
	ConnState constants.ConnState
	Meta      PlayerMeta

	closeOnce sync.Once
	conn      net.Conn
	sendCh    chan packet.Packet
	doneCh    chan struct{}
}

type PlayerMeta struct {
	RemoteAddr string
	User       string
	UserID     uuid.UUID
}

func (player *Player) SendChat(msg Chat) {
	chatMessage := packet.NewPacket(0x0F)
	chatMessage.WriteString(msg.String()).WriteRaw([]byte{0})
	player.Send(chatMessage)
}

func (player *Player) Send(pkt packet.Packet) {
	player.sendCh <- pkt
}

func (player *Player) Done() chan struct{} {
	return player.doneCh
}

func (player *Player) Close() (err error) {
	player.closeOnce.Do(func() {
		close(player.doneCh)
		err = player.conn.Close()
	})
	return err
}

func NewPlayer(conn net.Conn) *Player {
	player := &Player{
		conn: conn,
		Meta: PlayerMeta{
			RemoteAddr: conn.RemoteAddr().String(),
			User:       "",         // this should get from db
			UserID:     uuid.New(), // this should get from db
		},
		ConnState: constants.ConnStateInit,
		sendCh:    make(chan packet.Packet, 8),
		doneCh:    make(chan struct{}),
	}
	go func() {
	loop:
		for {
			select {
			case pkt := <-player.sendCh:
				// calculate pkt length and write back
				if _, err := pkt.WriteTo(conn); err != nil {
					log.Error(err)
					break loop
				}
			case <-player.doneCh:
				return
			}
			if player.ConnState == constants.ConnStateClose {
				break
			}
		}
		if err := player.Close(); err != nil {
			log.Error()
		}
	}()
	return player
}
