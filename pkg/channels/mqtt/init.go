package mqtt

import (
	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/channels"
	"github.com/xibodev/facet-studio/pkg/config"
)

func init() {
	channels.RegisterSafeFactory(
		config.ChannelMQTT,
		func(bc *config.Channel, cfg *config.MQTTSettings, b *bus.MessageBus) (channels.Channel, error) {
			return NewMQTTChannel(bc, cfg, b)
		},
	)
}
