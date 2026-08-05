package cmd

import "testing"

func TestServerCmd_DefaultEnvIsDev(t *testing.T) {
	cmd := newServerCommand()
	env, _ := cmd.Flags().GetString("env")
	if env != "" {
		t.Fatal("default env must be empty so runtime picks dev")
	}
}
