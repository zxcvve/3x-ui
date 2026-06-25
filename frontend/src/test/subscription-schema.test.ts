import { describe, expect, test } from 'vitest';

import {
  SubscriptionDetailSchema,
  SubscriptionSummaryListSchema,
  RoutedProfileRequestSchema,
} from '@/schemas/subscription';

describe('subscription schemas', () => {
  test('parses summary list and normalizes nullable arrays', () => {
    const parsed = SubscriptionSummaryListSchema.parse([
      {
        subId: 'shared-sub',
        memberCount: 2,
        enabledCount: 1,
        disabledCount: 1,
        traffic: { up: 10, down: 20, total: 100 },
        totalGB: { value: 0, mixed: true },
        expiryTime: { value: 0, mixed: true },
        inboundIds: null,
        inboundTags: null,
        inbounds: null,
      },
    ]);

    expect(parsed[0].inboundIds).toEqual([]);
    expect(parsed[0].inboundTags).toEqual([]);
    expect(parsed[0].inbounds).toEqual([]);
  });

  test('parses detail urls and member profiles', () => {
    const parsed = SubscriptionDetailSchema.parse({
      subId: 'detail-sub',
      memberCount: 1,
      enabledCount: 1,
      disabledCount: 0,
      traffic: { up: 1, down: 2, total: 3 },
      totalGB: { value: 3, mixed: false },
      expiryTime: { value: 0, mixed: false },
      inboundIds: [1],
      inboundTags: ['in'],
      inbounds: [{ id: 1, tag: 'in', remark: 'Inbound' }],
      urls: [{ format: 'raw', url: 'https://example.com/sub/detail-sub' }],
      members: [{
        id: 10,
        email: 'profile@example.com',
        enable: true,
        totalGB: 3,
        expiryTime: 0,
        inboundIds: [1],
        inbounds: [{ id: 1, tag: 'in', remark: 'Inbound' }],
        traffic: { up: 1, down: 2, total: 3 },
        routeTag: 'direct',
      }],
    });

    expect(parsed.urls[0].format).toBe('raw');
    expect(parsed.members[0].routeTag).toBe('direct');
  });

  test('validates routed profile requests', () => {
    expect(() => RoutedProfileRequestSchema.parse({
      email: 'profile@example.com',
      inboundIds: [1],
      outboundTag: 'direct',
    })).not.toThrow();

    expect(() => RoutedProfileRequestSchema.parse({
      email: '',
      inboundIds: [],
      outboundTag: '',
    })).toThrow();
  });
});
