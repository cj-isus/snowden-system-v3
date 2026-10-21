/** Set before mounting the app. Production never imports the fixture module. */
export let MOCK_ACTIVE = false
export function markMockPreview() { MOCK_ACTIVE = true }
