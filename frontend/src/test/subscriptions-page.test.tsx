import { describe, expect, test, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import { renderWithProviders } from './test-utils';

vi.mock('@/layouts/AppSidebar', () => ({ default: () => <aside data-testid="sidebar" /> }));
vi.mock('@/hooks/usePageTitle', () => ({ usePageTitle: () => undefined }));
vi.mock('@/hooks/useMediaQuery', () => ({ useMediaQuery: () => ({ isMobile: false }) }));
vi.mock('@/api/queries/useInboundOptions', () => ({
  useInboundOptions: () => ({ data: [{ id: 1, remark: 'Main inbound', tag: 'main-in' }] }),
}));
vi.mock('@/api/queries/useOutboundTags', () => ({
  useOutboundTags: () => ({ data: ['direct'] }),
}));
vi.mock('@/api/queries/useSubscriptionsQuery', async () => {
  const actual = await vi.importActual<typeof import('@/api/queries/useSubscriptionsQuery')>('@/api/queries/useSubscriptionsQuery');
  return {
    ...actual,
    useSubscriptionsQuery: () => ({
      subscriptions: [{
        subId: 'shared-sub',
        memberCount: 1,
        enabledCount: 1,
        disabledCount: 0,
        traffic: { up: 10, down: 20, total: 100 },
        totalGB: { value: 100, mixed: false },
        expiryTime: { value: 0, mixed: false },
        inboundIds: [1],
        inboundTags: ['main-in'],
        inbounds: [{ id: 1, tag: 'main-in', remark: 'Main inbound' }],
      }],
      loading: false,
      fetched: true,
      fetchError: '',
      refetch: vi.fn(),
    }),
    useSubscriptionDetailQuery: () => ({
      detail: {
        subId: 'shared-sub',
        memberCount: 1,
        enabledCount: 1,
        disabledCount: 0,
        traffic: { up: 10, down: 20, total: 100 },
        totalGB: { value: 100, mixed: false },
        expiryTime: { value: 0, mixed: false },
        inboundIds: [1],
        inboundTags: ['main-in'],
        inbounds: [{ id: 1, tag: 'main-in', remark: 'Main inbound' }],
        urls: [{ format: 'raw', url: 'https://example.com/sub/shared-sub' }],
        members: [{
          id: 1,
          email: 'profile@example.com',
          enable: true,
          totalGB: 100,
          expiryTime: 0,
          inboundIds: [1],
          inbounds: [{ id: 1, tag: 'main-in', remark: 'Main inbound' }],
          traffic: { up: 10, down: 20, total: 100 },
          routeTag: 'direct',
        }],
      },
      loading: false,
      fetched: true,
      fetchError: '',
      refetch: vi.fn(),
    }),
    useRoutedProfileMutation: () => ({
      mutateAsync: vi.fn().mockResolvedValue({ success: true, obj: { email: 'new@example.com' } }),
      isPending: false,
    }),
  };
});

import SubscriptionsPage from '@/pages/subscriptions/SubscriptionsPage';

describe('SubscriptionsPage', () => {
  test('renders subscription detail and opens routed-profile modal', async () => {
    renderWithProviders(<SubscriptionsPage />);

    expect(screen.getAllByText('shared-sub').length).toBeGreaterThan(0);
    expect(screen.getByText('profile@example.com')).toBeTruthy();
    expect(screen.getByText('https://example.com/sub/shared-sub')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /profile/i }));

    expect(await screen.findByText('Routed profile')).toBeTruthy();
    expect(screen.getByLabelText('Email')).toBeTruthy();
    expect(screen.getByLabelText('Inbounds')).toBeTruthy();
    expect(screen.getByLabelText('Outbound')).toBeTruthy();
  });
});
