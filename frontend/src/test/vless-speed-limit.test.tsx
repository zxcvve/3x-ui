import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import { createDefaultVlessClient } from '@/lib/xray/inbound-defaults';
import { VlessClientSchema } from '@/schemas/protocols/inbound/vless';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

describe('VLESS client speed limits', () => {
  it('keeps speed limits in VLESS client settings JSON', () => {
    const client = createDefaultVlessClient({
      email: 'limited@example.test',
      id: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02',
      speedLimitUpload: 1024,
      speedLimitDownload: 2048,
    });

    const parsed = VlessClientSchema.parse(client);
    expect(parsed.speedLimitUpload).toBe(1024);
    expect(parsed.speedLimitDownload).toBe(2048);
    expect(JSON.parse(JSON.stringify(parsed))).toMatchObject({
      speedLimitUpload: 1024,
      speedLimitDownload: 2048,
    });
  });

  it('shows speed-limit fields when editing a VLESS-attached client', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue({ success: true, msg: '', obj: [] });

    renderWithProviders(
      <ClientFormModal
        open
        mode="edit"
        client={{
          email: 'limited@example.test',
          uuid: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02',
          subId: 'sub',
          enable: true,
          speedLimitUpload: 1024,
          speedLimitDownload: 2048,
        }}
        inbounds={[{ id: 1, protocol: 'vless', tag: 'vless-in' }]}
        attachedIds={[1]}
        save={async () => ({ success: true })}
        onOpenChange={() => {}}
      />,
    );

    expect(await screen.findByText(/speedLimitUpload|Upload Speed Limit/)).toBeTruthy();
    expect(screen.getByText(/speedLimitDownload|Download Speed Limit/)).toBeTruthy();
  });
});
