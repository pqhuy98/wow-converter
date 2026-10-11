# Edge's component downloader creates service-owned BITS jobs that outlive Browser.close.
# Both launchers supply only the unique temp root created by the current capture.
$ErrorActionPreference = 'Stop'
try {
    if ([string]::IsNullOrWhiteSpace($env:WOW_REPORT_SHOT_TEMP)) {
        throw 'Missing capture temp directory'
    }
    $root = [IO.Path]::GetFullPath($env:WOW_REPORT_SHOT_TEMP).TrimEnd('\')
    $prefix = $root + '\'
    Import-Module BitsTransfer
    $cancelled = 0
    # Default enumeration is current-user only; never use -AllUsers or match a job's display name.
    foreach ($job in @(Get-BitsTransfer)) {
        $files = @($job.FileList)
        if ($files.Count -eq 0) { continue }
        $outside = @($files | Where-Object {
            -not [IO.Path]::GetFullPath($_.LocalName).StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)
        })
        # Cancellation affects every file in a job: mixed-destination jobs must be preserved.
        if ($outside.Count -eq 0) {
            Remove-BitsTransfer -BitsJob $job -Confirm:$false
            $cancelled++
        }
    }
    Write-Output $cancelled
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
