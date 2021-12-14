package main

import (
	"testing"
)

func Test_checkDeviceIp(t *testing.T) {
	type args struct {
		deviceIp string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid case",
			args: args{
				deviceIp: "123.123.123.123",
			},
			want: true,
		},
		{
			name: "valid case zero",
			args: args{
				deviceIp: "0.0.0.0",
			},
			want: true,
		},
		{
			name: "invalid 3 dot",
			args: args{
				deviceIp: "123.123.123",
			},
			want: false,
		},
		{
			name: "invalid string",
			args: args{
				deviceIp: "asdfdsfd",
			},
			want: false,
		},
		{
			name: "invalid empty",
			args: args{
				deviceIp: "",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDeviceIp(tt.args.deviceIp); got != tt.want {
				t.Errorf("checkDeviceIp() = %v, want %v", got, tt.want)
			}
		})
	}
}
