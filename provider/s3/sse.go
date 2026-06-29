package s3

import (
	"encoding/base64"

	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/oddbit-project/blueprint/utils"
)

// sseCustomerKey builds an SSE-C encryption from a base64-encoded 32-byte key.
func sseCustomerKey(b64key string) (encrypt.ServerSide, error) {
	key, err := base64.StdEncoding.DecodeString(b64key)
	if err != nil {
		return nil, utils.Error("invalid SSE-C customer key: " + err.Error())
	}
	return encrypt.NewSSEC(key)
}

// serverSideEncryption builds a minio encrypt.ServerSide from ObjectOptions.
// Returns (nil, nil) when no encryption is requested. A customer-provided key
// (SSE-C) takes precedence over ServerSideEncryption.
func serverSideEncryption(opts ObjectOptions) (encrypt.ServerSide, error) {
	if opts.SSECustomerKey != "" {
		return sseCustomerKey(opts.SSECustomerKey)
	}

	switch opts.ServerSideEncryption {
	case "":
		return nil, nil
	case SSEAlgorithmAES256:
		return encrypt.NewSSE(), nil
	case SSEAlgorithmKMS:
		var context interface{}
		if len(opts.SSEKMSEncryptionContext) > 0 {
			context = opts.SSEKMSEncryptionContext
		}
		return encrypt.NewSSEKMS(opts.SSEKMSKeyId, context)
	case SSEAlgorithmKMSDSSE:
		// minio-go's encrypt package has no DSSE type; fail rather than
		// silently downgrade to regular SSE-KMS.
		return nil, utils.Error("server-side encryption " + SSEAlgorithmKMSDSSE + " is not supported by the minio-go client")
	default:
		return nil, utils.Error("unsupported server-side encryption: " + opts.ServerSideEncryption)
	}
}
