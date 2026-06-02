export const stateColor: Record<
  string,
  'default' | 'primary' | 'info' | 'success' | 'error' | 'warning'
> = {
  PENDING: 'info',
  PROCESSING: 'primary',
  SUCCEEDED: 'success',
  FAILED: 'error',
  EXPIRED: 'warning',
  SKIPPED: 'default',
}
