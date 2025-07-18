package websocket

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/go-pantheon/fabrica-net/client"
	"github.com/go-pantheon/fabrica-net/internal"
	"github.com/go-pantheon/fabrica-net/websocket/frame"
	"github.com/go-pantheon/fabrica-net/websocket/wsconn"
	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/gorilla/websocket"
)

var _ internal.Dialer = (*Dialer)(nil)

type Dialer struct {
	cliID  int64
	url    string
	dialer *websocket.Dialer
	origin string
}

func newDialer(cliID int64, url string, origin string) *Dialer {
	return &Dialer{
		cliID:  cliID,
		url:    url,
		origin: origin,
		dialer: &websocket.Dialer{
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

func (d *Dialer) Dial(ctx context.Context, target string) ([]internal.ConnCarrier, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, errors.Wrapf(err, "parse url failed. url=%s", target)
	}

	header := http.Header{}
	header.Set("Origin", d.origin)

	c, resp, err := d.dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return nil, errors.Wrapf(err, "connect failed. url=%s", target)
	}

	defer func() {
		if resp != nil {
			if bodyErr := resp.Body.Close(); bodyErr != nil {
				err = errors.Join(err, errors.Wrapf(bodyErr, "close response body failed"))
			}
		}
	}()

	conn := wsconn.NewWebSocketConn(c)
	codec := frame.New(conn)

	return []internal.ConnCarrier{internal.NewConnCarrier(client.DialogID(d.cliID, 0), conn, codec)}, nil
}

func (d *Dialer) Stop(ctx context.Context) error {
	return nil
}

func (d *Dialer) Target() string {
	return d.url
}
