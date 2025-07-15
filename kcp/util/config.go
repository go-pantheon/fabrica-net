package util

import (
	"time"

	"github.com/go-pantheon/fabrica-net/conf"
	"github.com/go-pantheon/fabrica-util/errors"
	kcpgo "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
)

type ConfigValidator struct{}

func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

func (v *ConfigValidator) Validate(conf conf.KCP) error {
	if conf.MTU < 576 || conf.MTU > 1500 {
		return errors.Errorf("invalid MTU: %d, must be between 576 and 1500", conf.MTU)
	}

	if conf.DataShards < 0 || conf.DataShards > 255 {
		return errors.Errorf("invalid DataShards: %d, must be between 0 and 255", conf.DataShards)
	}

	if conf.ParityShards < 0 || conf.ParityShards > 255 {
		return errors.Errorf("invalid ParityShards: %d, must be between 0 and 255", conf.ParityShards)
	}

	if conf.WindowSize[0] <= 0 || conf.WindowSize[1] <= 0 {
		return errors.Errorf("invalid WindowSize: %v, both send and receive windows must be positive", conf.WindowSize)
	}

	if conf.SmuxStreamSize <= 0 {
		return errors.Errorf("invalid SmuxStreamSize: %d, must be positive", conf.SmuxStreamSize)
	}

	if conf.KeepAliveInterval <= 0 {
		return errors.Errorf("invalid KeepAliveInterval: %v, must be positive", conf.KeepAliveInterval)
	}

	if conf.KeepAliveTimeout <= conf.KeepAliveInterval {
		return errors.Errorf("KeepAliveTimeout (%v) must be greater than KeepAliveInterval (%v)",
			conf.KeepAliveTimeout, conf.KeepAliveInterval)
	}

	return nil
}

type ConnConfigurer struct {
	conf conf.KCP
}

// NewConnConfigurer creates a new connection configurer
func NewConnConfigurer(conf conf.KCP) *ConnConfigurer {
	return &ConnConfigurer{conf: conf}
}

func (c *ConnConfigurer) ConfigureConnection(conn *kcpgo.UDPSession) {
	// Set KCP protocol parameters
	conn.SetNoDelay(c.conf.NoDelay[0], c.conf.NoDelay[1], c.conf.NoDelay[2], c.conf.NoDelay[3])
	conn.SetWindowSize(c.conf.WindowSize[0], c.conf.WindowSize[1])
	conn.SetMtu(c.conf.MTU)
	conn.SetACKNoDelay(c.conf.ACKNoDelay)
	conn.SetWriteDelay(c.conf.WriteDelay)
}

func (c *ConnConfigurer) CreateSmuxConfig() *smux.Config {
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = c.conf.KeepAliveInterval
	smuxConfig.KeepAliveTimeout = c.conf.KeepAliveTimeout
	smuxConfig.MaxFrameSize = c.conf.MaxFrameSize
	smuxConfig.MaxReceiveBuffer = c.conf.MaxReceiveBuffer

	return smuxConfig
}

// AdaptiveConfigTuner provides dynamic configuration tuning based on network conditions
type AdaptiveConfigTuner struct {
	baseConfig    conf.KCP
	currentConfig conf.KCP
	lastTuned     time.Time

	// Network condition metrics
	rttSamples []time.Duration
	packetLoss float64
	throughput uint64

	// Tuning parameters
	maxRTTSamples  int
	tuningInterval time.Duration
}

func NewAdaptiveConfigTuner(baseConfig conf.KCP) *AdaptiveConfigTuner {
	return &AdaptiveConfigTuner{
		baseConfig:     baseConfig,
		currentConfig:  baseConfig,
		lastTuned:      time.Now(),
		maxRTTSamples:  100,
		tuningInterval: 30 * time.Second,
		rttSamples:     make([]time.Duration, 0, 100),
	}
}

func (t *AdaptiveConfigTuner) AddRTTSample(rtt time.Duration) {
	t.rttSamples = append(t.rttSamples, rtt)
	if len(t.rttSamples) > t.maxRTTSamples {
		t.rttSamples = t.rttSamples[1:]
	}
}

func (t *AdaptiveConfigTuner) UpdatePacketLoss(loss float64) {
	t.packetLoss = loss
}

func (t *AdaptiveConfigTuner) UpdateThroughput(throughput uint64) {
	t.throughput = throughput
}

func (t *AdaptiveConfigTuner) TuneConfig() conf.KCP {
	now := time.Now()
	if now.Sub(t.lastTuned) < t.tuningInterval {
		return t.currentConfig
	}

	t.lastTuned = now
	newConfig := t.baseConfig

	// Calculate average RTT
	if len(t.rttSamples) > 0 {
		var totalRTT time.Duration
		for _, rtt := range t.rttSamples {
			totalRTT += rtt
		}
		avgRTT := totalRTT / time.Duration(len(t.rttSamples))

		if avgRTT > 100*time.Millisecond {
			// High latency network
			newConfig.NoDelay = [4]int{0, 20, 2, 0}
			newConfig.WindowSize = [2]int{512, 512}
		} else if avgRTT < 20*time.Millisecond {
			// Low latency network
			newConfig.NoDelay = [4]int{1, 5, 2, 1}
			newConfig.WindowSize = [2]int{128, 128}
		}
	}

	// Adjust based on packet loss
	if t.packetLoss > 0.05 {
		if newConfig.DataShards > 0 && newConfig.ParityShards < 5 {
			newConfig.ParityShards = minInt(newConfig.ParityShards+1, 5)
		}
	} else if t.packetLoss < 0.01 {
		if newConfig.ParityShards > 1 {
			newConfig.ParityShards = maxInt(newConfig.ParityShards-1, 1)
		}
	}

	// Adjust based on throughput requirements
	if t.throughput > 10*1024*1024 {
		newConfig.MTU = 1400
		newConfig.WindowSize = [2]int{512, 512}
	}

	t.currentConfig = newConfig
	return newConfig
}

func (t *AdaptiveConfigTuner) GetCurrentConfig() conf.KCP {
	return t.currentConfig
}

func (t *AdaptiveConfigTuner) Reset() {
	t.currentConfig = t.baseConfig
	t.rttSamples = t.rttSamples[:0]
	t.packetLoss = 0
	t.throughput = 0
	t.lastTuned = time.Now()
}

// Helper functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
