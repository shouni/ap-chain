package builder

import (
	"fmt"

	"github.com/shouni/go-remote-io/remoteio"

	"ap-chain/internal/app"
)

// buildRemoteIO は、I/O コンポーネントを初期化します。
func buildRemoteIO(storage remoteio.IOFactory) (*app.RemoteIO, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage factory cannot be nil")
	}

	w, err := storage.OutputWriter()
	if err != nil {
		return nil, wrapInitErr("OutputWriter", err)
	}
	s, err := storage.URLSigner()
	if err != nil {
		return nil, wrapInitErr("URLSigner", err)
	}

	return &app.RemoteIO{
		Factory: storage,
		Writer:  w,
		Signer:  s,
	}, nil
}
