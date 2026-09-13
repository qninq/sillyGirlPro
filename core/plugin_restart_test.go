package core

import "testing"

func TestPluginRestartDecisionEscalatesToAlert(t *testing.T) {
	uuid := "restart-decision-test"
	clearPluginCrashState(uuid)

	for i := 1; i <= pluginRestartMaxRetries; i++ {
		count, alert, retry := pluginRestartDecision(uuid)
		if !retry || alert {
			t.Fatalf("crash %d should schedule a retry, got retry=%v alert=%v", i, retry, alert)
		}
		if count != i {
			t.Fatalf("crash count = %d; want %d", count, i)
		}
	}
	_, alert, retry := pluginRestartDecision(uuid)
	if retry {
		t.Fatal("crash beyond max retries must not schedule another restart")
	}
	if !alert {
		t.Fatal("first crash beyond max retries must alert")
	}
	if _, alertAgain, _ := pluginRestartDecision(uuid); alertAgain {
		t.Fatal("repeat crashes beyond max retries must not re-alert")
	}

	clearPluginCrashState(uuid)
	if _, _, retry = pluginRestartDecision(uuid); !retry {
		t.Fatal("clearing crash state should allow retries again")
	}
	clearPluginCrashState(uuid)
}
