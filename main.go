package rtmp

import (
	"context"
	"net/http"
	"strconv"

	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/config"
	"m7s.live/engine/v4/util"
)

type RTMPConfig struct {
	config.HTTP
	config.Publish
	config.Subscribe
	config.TCP
	config.Pull
	config.Push
	ChunkSize int  `default:"65535" desc:"分片大小"`
	KeepAlive bool `desc:"保持连接，流断开不关闭连接"` //保持rtmp连接，默认随着stream的close而主动断开
	C2        bool `desc:"握手是否检查C2"`
}

func pull(streamPath, url string) {
	if err := RTMPPlugin.Pull(streamPath, url, new(RTMPPuller), 0); err != nil {
		RTMPPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
	}
}
func (c *RTMPConfig) OnEvent(event any) {
	switch v := event.(type) {
	case FirstConfig:
		for streamPath, url := range c.PullOnStart {
			pull(streamPath, url)
		}
	case config.Config:
		RTMPPlugin.CancelFunc()
		if c.TCP.ListenAddr != "" {
			RTMPPlugin.Context, RTMPPlugin.CancelFunc = context.WithCancel(Engine)
			RTMPPlugin.Info("server rtmp start at", zap.String("listen addr", c.TCP.ListenAddr))
			go c.ListenTCP(RTMPPlugin, c)
		}
	case SEpublish:
		if remoteURL := conf.CheckPush(v.Target.Path); remoteURL != "" {
			if err := RTMPPlugin.Push(v.Target.Path, remoteURL, new(RTMPPusher), false); err != nil {
				RTMPPlugin.Error("push", zap.String("streamPath", v.Target.Path), zap.String("url", remoteURL), zap.Error(err))
			}
		}
	case InvitePublish: //按需拉流
		if remoteURL := conf.CheckPullOnSub(v.Target); remoteURL != "" {
			pull(v.Target, remoteURL)
		}
	}
}

var conf = &RTMPConfig{
	TCP: config.TCP{ListenAddr: ":1935"},
}

var RTMPPlugin = InstallPlugin(conf)

func filterStreams() (ss []*Stream) {
	Streams.Range(func(key string, s *Stream) {
		switch s.Publisher.(type) {
		case *RTMPReceiver, *RTMPPuller:
			ss = append(ss, s)
		}
	})
	return
}

func (*RTMPConfig) API_list(w http.ResponseWriter, r *http.Request) {
	util.ReturnFetchValue(filterStreams, w, r)
}

func (*RTMPConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	save, _ := strconv.Atoi(r.URL.Query().Get("save"))
	err := RTMPPlugin.Pull(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(RTMPPuller), save)
	if err != nil {
		util.ReturnError(util.APIErrorQueryParse, err.Error(), rw, r)
	} else {
		util.ReturnOK(rw, r)
	}
}

func (*RTMPConfig) API_Push(rw http.ResponseWriter, r *http.Request) {
	err := RTMPPlugin.Push(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(RTMPPusher), r.URL.Query().Has("save"))
	if err != nil {
		util.ReturnError(util.APIErrorQueryParse, err.Error(), rw, r)
	} else {
		util.ReturnOK(rw, r)
	}
}
