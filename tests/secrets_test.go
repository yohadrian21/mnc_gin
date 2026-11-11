package tests

import (
	"testing"

	"github.com/Massad/gin-boilerplate/models"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSecretsRejectsWeakValues(t *testing.T) {
	for name, secrets := range map[string]struct{ access, refresh string }{
		"missing access":     {"", testRefreshSecret},
		"missing refresh":    {testAccessSecret, ""},
		"placeholder":        {"CHANGE_ME_access_secret_openssl_rand_base64_48", testRefreshSecret},
		"shorter than 32":    {"only-22-characters-xyz", testRefreshSecret},
		"shared by both JWT": {testAccessSecret, testAccessSecret},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ACCESS_SECRET", secrets.access)
			t.Setenv("REFRESH_SECRET", secrets.refresh)
			assert.Error(t, models.ValidateSecrets())
		})
	}

	t.Setenv("ACCESS_SECRET", testAccessSecret)
	t.Setenv("REFRESH_SECRET", testRefreshSecret)
	t.Setenv("PAYMENT_WEBHOOK_SECRET", "")
	assert.NoError(t, models.ValidateSecrets(), "an unset webhook secret only disables the webhook")

	for _, weak := range []string{"short-webhook-key", "CHANGE_ME_webhook_secret_openssl_rand_base64_48"} {
		t.Setenv("PAYMENT_WEBHOOK_SECRET", weak)
		assert.Error(t, models.ValidateSecrets(), weak)
	}
	t.Setenv("PAYMENT_WEBHOOK_SECRET", testWebhookSecret)
	assert.NoError(t, models.ValidateSecrets())
}

// The sample env file is public, so it must never carry values the server would accept.
func TestSampleEnvFileOnlyHasPlaceholderSecrets(t *testing.T) {
	sample, err := godotenv.Read("../.env_copy")
	require.NoError(t, err)

	for _, key := range []string{"ACCESS_SECRET", "REFRESH_SECRET", "REDIS_SECRET"} {
		assert.Regexp(t, "^CHANGE_ME", sample[key], key)
	}
	assert.Empty(t, sample["PAYMENT_WEBHOOK_SECRET"])

	t.Setenv("ACCESS_SECRET", sample["ACCESS_SECRET"])
	t.Setenv("REFRESH_SECRET", sample["REFRESH_SECRET"])
	assert.Error(t, models.ValidateSecrets())
}
