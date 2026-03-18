import { test, expect } from '@playwright/test';

test('basic flow: host creates event and participant joins', async ({ page, context }) => {
  // 1. Host creates an event
  await page.goto('/');
  await page.fill('input[placeholder*="Title"]', 'E2E Test Event');
  await page.click('button:has-text("Create Event")');

  // Wait for navigation to dashboard
  await expect(page).toHaveURL(/\/event\/[0-9a-f-]+\/dashboard/);
  const eventUrl = page.url();
  const match = eventUrl.match(/\/event\/([0-9a-f-]+)\/dashboard/);
  if (!match) throw new Error('Could not find event ID in URL');
  const eventID = match[1];
  
  await expect(page.locator('h1')).toContainText('E2E Test Event');

  // 2. Participant joins in a new context (to simulate a different user)
  const participantPage = await context.newPage();
  await participantPage.goto(`/join/${eventID}`);

  await expect(participantPage.locator('h1')).toContainText('E2E Test Event');
  
  // Set nickname if required/optional
  const nicknameInput = participantPage.locator('input[placeholder*="Nickname"]');
  if (await nicknameInput.isVisible()) {
    await nicknameInput.fill('Participant1');
    await participantPage.click('button:has-text("Join")');
  }

  // 3. Host creates a poll
  await page.click('button:has-text("Create Poll")');
  await page.fill('input[placeholder="Poll Title"]', 'Favorite Color?');
  await page.fill('input[placeholder="Option 1"]', 'Red');
  await page.fill('input[placeholder="Option 2"]', 'Blue');
  await page.click('button:has-text("Save Poll")');

  // 4. Host switches to the new poll
  await page.click('button:has-text("Switch")');

  // 5. Participant sees the poll and votes
  await expect(participantPage.locator('text=Favorite Color?')).toBeVisible();
  await participantPage.click('label:has-text("Red")');
  await participantPage.click('button:has-text("Vote")');

  await expect(participantPage.locator('text=Voted')).toBeVisible();

  // 6. Host sees the vote results
  await expect(page.locator('text=Red')).toBeVisible();
  // Check if count updated (this depends on UI implementation)
});
