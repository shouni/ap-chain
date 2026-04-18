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

	w, err := storage.Writer()
	if err != nil {
		return nil, fmt.Errorf("failed to create output writer: %w", err)
	}
	s, err := storage.URLSigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create URL signer: %w", err)
	}

	return &app.RemoteIO{
		Factory: storage,
		Writer:  w,
		Signer:  s,
	}, nil
}
