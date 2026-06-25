import { describe, expect, test, vi } from 'vitest';
import { fireEvent, renderHook, screen, waitFor } from '@testing-library/react';
import type { ComponentType, ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import NodeFormModal from '@/pages/nodes/NodeFormModal';
import SubscriptionOutbounds from '@/pages/xray/outbounds/SubscriptionOutbounds';
import OutboundsTab from '@/pages/xray/outbounds/OutboundsTab';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

import { renderWithProviders } from './test-utils';

function queryWrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('node outbound bridging', () => {
  test('node form saves bridge enablement and selected inbound tags separately from outboundTag', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <NodeFormModal
        open
        mode="edit"
        node={{
          id: 7,
          name: 'de-node',
          scheme: 'https',
          address: 'node.example.com',
          port: 2053,
          basePath: '/',
          apiToken: 'token',
          enable: true,
          allowPrivateAddress: false,
          tlsVerifyMode: 'verify',
          inboundSyncMode: 'all',
          inboundTags: [],
          outboundTag: 'panel-egress',
          outboundBridgeEnable: true,
          outboundBridgeTags: ['remote-vless'],
        }}
        testConnection={vi.fn().mockResolvedValue({ success: true, obj: { status: 'online' } })}
        fetchFingerprint={vi.fn()}
        fetchInbounds={vi.fn().mockResolvedValue({ success: true, obj: [] })}
        save={save}
        provision={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    );

    expect(screen.getByLabelText('Expose inbounds as outbounds')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    const payload = save.mock.calls[0][0];
    expect(payload.outboundTag).toBe('panel-egress');
    expect(payload.outboundBridgeEnable).toBe(true);
    expect(payload.outboundBridgeTags).toEqual(['remote-vless']);
  });

  test('node form includes bridge selections in selected inbound sync payload', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <NodeFormModal
        open
        mode="edit"
        node={{
          id: 7,
          name: 'de-node',
          scheme: 'https',
          address: 'node.example.com',
          port: 2053,
          basePath: '/',
          apiToken: 'token',
          enable: true,
          allowPrivateAddress: false,
          tlsVerifyMode: 'verify',
          inboundSyncMode: 'selected',
          inboundTags: ['already-synced'],
          outboundBridgeEnable: true,
          outboundBridgeTags: ['remote-vless'],
        }}
        testConnection={vi.fn().mockResolvedValue({ success: true, obj: { status: 'online' } })}
        fetchFingerprint={vi.fn()}
        fetchInbounds={vi.fn().mockResolvedValue({ success: true, obj: [] })}
        save={save}
        provision={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    const payload = save.mock.calls[0][0];
    expect(payload.inboundSyncMode).toBe('selected');
    expect(payload.inboundTags).toEqual(expect.arrayContaining(['already-synced', 'remote-vless']));
    expect(payload.outboundBridgeTags).toEqual(['remote-vless']);
  });

  test('read-only generated rows show node source metadata', () => {
    renderWithProviders(
      <SubscriptionOutbounds
        subscriptionOutbounds={[{
          nodeName: 'de-node',
          sourceInboundTag: 'remote-vless',
          outbound: {
            tag: 'node-7-remote-vless',
            protocol: 'vless',
            settings: { vnext: [{ address: 'node.example.com', port: 443 }] },
          },
        }]}
        title="From node inbounds (read-only)"
        description="Generated from enabled node bridge selections."
        outboundsTraffic={[]}
        subscriptionTestStates={{}}
        testMode="tcp"
        isMobile={false}
        onTestSubscription={vi.fn()}
      />,
    );

    expect(screen.getByText('From node inbounds (read-only)')).toBeTruthy();
    expect(screen.getByText('node-7-remote-vless')).toBeTruthy();
    expect(screen.getByText('de-node / remote-vless')).toBeTruthy();
  });

  test('outbound tag query includes node-derived tags', async () => {
    vi.mocked(HttpUtil.post).mockResolvedValueOnce({
      success: true,
      msg: '',
      obj: JSON.stringify({
        xraySetting: {
          outbounds: [{ tag: 'direct', protocol: 'freedom' }],
          routing: { balancers: [] },
        },
        subscriptionOutboundTags: ['sub1-relay'],
        nodeOutboundTags: ['node-7-remote-vless'],
      }),
    });

    const { result } = renderHook(() => useOutboundTags(), { wrapper: queryWrapper });

    await waitFor(() => expect(result.current.data).toContain('node-7-remote-vless'));
    expect(result.current.data).toEqual(expect.arrayContaining(['direct', 'sub1-relay', 'node-7-remote-vless']));
  });

  test('outbounds tab shows unavailable node bridge selections with reasons', () => {
    const Tab = OutboundsTab as unknown as ComponentType<Record<string, unknown>>;
    renderWithProviders(
      <Tab
        templateSettings={{ outbounds: [{ tag: 'direct', protocol: 'freedom' }], routing: { rules: [], balancers: [] } }}
        setTemplateSettings={vi.fn()}
        outboundsTraffic={[]}
        outboundTestStates={{}}
        subscriptionTestStates={{}}
        testingAll={false}
        inboundTags={[]}
        subscriptionOutbounds={[]}
        subscriptionOutboundTags={[]}
        nodeOutbounds={[]}
        nodeOutboundTags={[]}
        nodeOutboundCandidates={[{
          nodeName: 'de-node',
          sourceInboundTag: 'remote-vless',
          tag: 'node-7-remote-vless',
          protocol: 'vless',
          available: false,
          unavailableReason: 'missing_credentials',
        }]}
        isMobile={false}
        onResetTraffic={vi.fn()}
        onTest={vi.fn()}
        onTestSubscription={vi.fn()}
        onTestAll={vi.fn()}
        onShowWarp={vi.fn()}
        onShowNord={vi.fn()}
      />,
    );

    expect(screen.getByText('node-7-remote-vless')).toBeTruthy();
    expect(screen.getByText('de-node / remote-vless')).toBeTruthy();
    expect(screen.getByText('Missing credentials')).toBeTruthy();
  });
});
