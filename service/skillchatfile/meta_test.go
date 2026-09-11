package skillchatfile

import "testing"

func TestIsSupportedChatFileExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{ext: "csv", want: true},
		{ext: "md", want: true},
		{ext: "txt", want: true},
		{ext: "CSV", want: true},
		{ext: " pdf ", want: false},
		{ext: "xlsx", want: false},
		{ext: "", want: false},
	}

	if ChatFileUploadMaxBytes != 50*1024*1024 {
		t.Fatalf("ChatFileUploadMaxBytes = %d, want 50MB", ChatFileUploadMaxBytes)
	}

	for _, test := range tests {
		t.Run(test.ext, func(t *testing.T) {
			if got := IsSupportedChatFileExtension(test.ext); got != test.want {
				t.Fatalf("IsSupportedChatFileExtension(%q) = %v, want %v", test.ext, got, test.want)
			}
		})
	}
}
