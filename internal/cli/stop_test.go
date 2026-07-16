package cli

import "testing"

func TestDecideStopAction(t *testing.T) {
	tests := []struct {
		name          string
		info          *pidInfo
		serverRunning bool
		pidAlive      bool
		want          stopAction
	}{
		{
			name:          "our server running",
			info:          &pidInfo{Port: 8080, PID: 12345},
			serverRunning: true,
			pidAlive:      true,
			want:          stopKill,
		},
		{
			name:          "server running but process dead (race)",
			info:          &pidInfo{Port: 8080, PID: 12345},
			serverRunning: true,
			pidAlive:      false,
			want:          stopKill,
		},
		{
			name:          "pid alive but not our server",
			info:          &pidInfo{Port: 8080, PID: 12345},
			serverRunning: false,
			pidAlive:      true,
			want:          stopSkip,
		},
		{
			name:          "process dead",
			info:          &pidInfo{Port: 8080, PID: 12345},
			serverRunning: false,
			pidAlive:      false,
			want:          stopStale,
		},
		{
			name:          "legacy pid file port=0 process alive",
			info:          &pidInfo{Port: 0, PID: 12345},
			serverRunning: false,
			pidAlive:      true,
			want:          stopLegacy,
		},
		{
			name:          "legacy pid file port=0 process dead",
			info:          &pidInfo{Port: 0, PID: 12345},
			serverRunning: false,
			pidAlive:      false,
			want:          stopLegacy,
		},
		{
			name:          "legacy pid file ignores serverRunning=true",
			info:          &pidInfo{Port: 0, PID: 12345},
			serverRunning: true,
			pidAlive:      true,
			want:          stopLegacy,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideStopAction(tc.info, tc.serverRunning, tc.pidAlive)
			if got != tc.want {
				t.Errorf("decideStopAction(Port=%d, PID=%d, running=%v, alive=%v) = %d, want %d",
					tc.info.Port, tc.info.PID, tc.serverRunning, tc.pidAlive, got, tc.want)
			}
		})
	}
}
