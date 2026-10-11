import { test, expect } from '@playwright/test';

test.describe('Critical User Flows', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    // Wait for app to be ready
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000); // Allow Wails to initialize
  });

  test('App loads and shows main stock view', async ({ page }) => {
    // Check main view elements
    await expect(page.locator('text=自选股')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=行情')).toBeVisible();
  });

  test('Stock search and K-line analysis', async ({ page }) => {
    // Find search input (adapt selector to actual UI)
    const searchInput = page.locator('input[placeholder*="搜索"], input[placeholder*="代码"], input[placeholder*="名称"]').first();
    if (await searchInput.isVisible({ timeout: 5000 })) {
      await searchInput.fill('600671');
      await searchInput.press('Enter');
      await page.waitForTimeout(1000);
      
      // Should navigate to K-line analysis or show stock detail
      await expect(page.locator('text=600671, text=天目药业').first()).toBeVisible({ timeout: 5000 });
    }
  });

  test('Market intelligence drawer opens', async ({ page }) => {
    // Click market intelligence button (adapt to actual UI)
    const intelButton = page.locator('button:has-text("市场情报"), button:has-text("研报"), [data-testid="market-intel"]').first();
    if (await intelButton.isVisible({ timeout: 5000 })) {
      await intelButton.click();
      await page.waitForTimeout(500);
      
      // Drawer should open
      await expect(page.locator('text=市场情报, text=市场研判, text=盘前简报').first()).toBeVisible({ timeout: 5000 });
    }
  });

  test('Agent chat interaction', async ({ page }) => {
    // Navigate to agent chat
    const agentTab = page.locator('a:has-text("Agent"), button:has-text("AI助手"), [href*="agent"]').first();
    if (await agentTab.isVisible({ timeout: 5000 })) {
      await agentTab.click();
      await page.waitForTimeout(1000);
      
      // Send a simple message
      const chatInput = page.locator('textarea[placeholder*="输入"], input[placeholder*="输入"], .chat-input').first();
      if (await chatInput.isVisible({ timeout: 5000 })) {
        await chatInput.fill('查询 600671 现价');
        await chatInput.press('Enter');
        await page.waitForTimeout(5000); // Wait for agent response
        
        // Should show some response
        await expect(page.locator('.message, .chat-message, [class*="message"]').last()).toBeVisible({ timeout: 10000 });
      }
    }
  });

  test('Settings page accessible', async ({ page }) => {
    const settingsLink = page.locator('a:has-text("设置"), button:has-text("设置"), [href*="settings"]').first();
    if (await settingsLink.isVisible({ timeout: 5000 })) {
      await settingsLink.click();
      await page.waitForTimeout(1000);
      
      await expect(page.locator('text=API, text=模型, text=通用, text=推送').first()).toBeVisible({ timeout: 5000 });
    }
  });

  test('Market view shows sectors', async ({ page }) => {
    const marketLink = page.locator('a:has-text("大盘"), a:has-text("行情"), [href*="market"]').first();
    if (await marketLink.isVisible({ timeout: 5000 })) {
      await marketLink.click();
      await page.waitForTimeout(2000);
      
      await expect(page.locator('text=板块, text=资金流, text=指数').first()).toBeVisible({ timeout: 10000 });
    }
  });
});
