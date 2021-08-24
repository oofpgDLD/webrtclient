package main

import (
	"flag"
	"fmt"
	rtp_to_webrtc "github.com/oofpgDLD/webrtclient/example/client/rtp-to-webrtc"
	"github.com/oofpgDLD/webrtclient/example/client/signal/testsignal/client"
	"github.com/oofpgDLD/webrtclient/internal/signal"
	"github.com/pion/webrtc/v3"
	"net/url"
)

var addr = flag.String("addr", "172.16.101.131:19801", "http service address")
var u = url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}

func main() {
	flag.Parse()
	engine := &webrtc.MediaEngine{}
	if err := engine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeVP8,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType:        0,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	if err := engine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeOpus,
			ClockRate:    48000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType:        0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	//初始化 信令服务
	c := client.NewClient("rtp-forwarder", u)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(engine))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	//
	rtp_to_webrtc.RTPToWebInit(peerConnection, "127.0.0.1", 5004)

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	//signal.Decode(signal.MustReadStdin(), &offer)
	signal.Decode(string(c.Read()), &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	fmt.Println("ICE Gathering start")
	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete
	fmt.Println("ICE Gathering complete")

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))
	err = client.Put("http://"+ *addr + "/pub", "demo", signal.Encode(*peerConnection.LocalDescription()))
	if err != nil {
		fmt.Println("put sdp err:", err.Error())
	}

	go rtp_to_webrtc.ServeVideoTrack()
	// Block forever
	select {}
}