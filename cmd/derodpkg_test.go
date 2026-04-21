package derodpkg

import (
	"testing"
)

func TestNewDaemon_ValidParams(t *testing.T) {
	params := map[string]interface{}{
		"--rpc-bind": "127.0.0.1:20202",
		"--testnet":  true,
	}

	d, err := NewDaemon(params)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil Daemon")
	}
	if d.Initialized() {
		t.Error("expected initialized to be false initially")
	}
	if d.Started() {
		t.Error("expected started to be false initially")
	}
}

func TestNewDaemon_NilParams(t *testing.T) {
	d, err := NewDaemon(nil)
	if err != nil {
		t.Fatalf("NewDaemon with nil params failed: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil Daemon")
	}
	if d.Params() == nil {
		t.Error("expected params to be initialized")
	}
}

func TestDaemon_StopNil(t *testing.T) {
	var d *Daemon
	if err := d.Stop(); err != nil {
		t.Errorf("Stop on nil Daemon should not error, got: %v", err)
	}
}

func TestDaemon_StopBeforeInit(t *testing.T) {
	d, err := NewDaemon(nil)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}
	if err := d.Stop(); err != nil {
		t.Errorf("Stop before init should not error, got: %v", err)
	}
}

func TestDaemon_StartBeforeInit(t *testing.T) {
	d, err := NewDaemon(nil)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}
	if err := d.Start(); err == nil {
		t.Error("expected error when Start is called before Initialize")
	}
}

func TestDaemon_ParamAccessors(t *testing.T) {
	params := map[string]interface{}{
		"--rpc-bind": "127.0.0.1:20202",
	}

	d, err := NewDaemon(params)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}

	if v, ok := d.GetParam("--rpc-bind"); !ok || v != "127.0.0.1:20202" {
		t.Error("expected --rpc-bind param to exist")
	}

	d.SetParam("--testnet", true)
	if v, ok := d.GetParam("--testnet"); !ok || v != true {
		t.Error("expected --testnet param to be set")
	}

	d.RemoveParam("--testnet")
	if _, ok := d.GetParam("--testnet"); ok {
		t.Error("expected --testnet param to be removed")
	}
}
