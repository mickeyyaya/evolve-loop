package credsource

import (
	"context"
	"fmt"
)

const credReadScript = `$ErrorActionPreference = 'Stop'
Add-Type -Namespace EvolveLoop -Name Cred -MemberDefinition '[DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)] public static extern bool CredReadW(string target, int type, int flags, out IntPtr cred); [DllImport("advapi32.dll")] public static extern void CredFree(IntPtr cred);'
$cred = [IntPtr]::Zero
if (-not [EvolveLoop.Cred]::CredReadW('%s', 1, 0, [ref]$cred)) { exit 2 }
try {
  $size = [IntPtr]::Size
  $blobSize = [Runtime.InteropServices.Marshal]::ReadInt32($cred, 8 + 2 * $size + 8)
  if ($blobSize -eq 0) { exit 3 }
  $blob = [Runtime.InteropServices.Marshal]::ReadIntPtr($cred, 8 + 3 * $size + 8)
  [Runtime.InteropServices.Marshal]::PtrToStringUni($blob, [int]($blobSize / 2))
} finally {
  [EvolveLoop.Cred]::CredFree($cred)
}`

type credentialManager struct {
	run Runner
}

func (credentialManager) Name() string { return "credential-manager" }

func (c credentialManager) Lookup(ctx context.Context, host string) (Answer, error) {
	script := fmt.Sprintf(credReadScript, Service+":"+host)
	out, err := c.run(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return Answer{}, fmt.Errorf("%s: %w", c.Name(), err)
	}
	return secretAnswer(c.Name(), out)
}
