import { z } from 'zod';

import { ClientTrafficSchema } from '@/schemas/client';

const nullableStringArray = z.array(z.string()).nullable().transform((v) => v ?? []);
const nullableNumberArray = z.array(z.number()).nullable().transform((v) => v ?? []);

export const SubscriptionMixedNumberSchema = z.object({
  value: z.number(),
  mixed: z.boolean(),
});

export const SubscriptionInboundRefSchema = z.object({
  id: z.number(),
  tag: z.string().optional().default(''),
  remark: z.string().optional().default(''),
}).loose();

const nullableInboundRefs = z.array(SubscriptionInboundRefSchema).nullable().transform((v) => v ?? []);

export const SubscriptionUrlSchema = z.object({
  format: z.enum(['raw', 'json', 'clash']),
  url: z.string(),
});

export const SubscriptionMemberSchema = z.object({
  id: z.number(),
  email: z.string(),
  enable: z.boolean(),
  totalGB: z.number().optional().default(0),
  expiryTime: z.number().optional().default(0),
  inboundIds: nullableNumberArray.optional(),
  inbounds: nullableInboundRefs.optional(),
  traffic: ClientTrafficSchema.optional().default({}),
  routeTag: z.string().optional().default(''),
}).loose();

export const SubscriptionSummarySchema = z.object({
  subId: z.string(),
  memberCount: z.number(),
  enabledCount: z.number(),
  disabledCount: z.number(),
  traffic: ClientTrafficSchema.optional().default({}),
  totalGB: SubscriptionMixedNumberSchema,
  expiryTime: SubscriptionMixedNumberSchema,
  inboundIds: nullableNumberArray.optional(),
  inboundTags: nullableStringArray.optional(),
  inbounds: nullableInboundRefs.optional(),
}).loose();

export const SubscriptionSummaryListSchema = z.array(SubscriptionSummarySchema).nullable().transform((v) => v ?? []);

export const SubscriptionDetailSchema = SubscriptionSummarySchema.extend({
  urls: z.array(SubscriptionUrlSchema).nullable().transform((v) => v ?? []),
  members: z.array(SubscriptionMemberSchema).nullable().transform((v) => v ?? []),
}).loose();

export const RoutedProfileRequestSchema = z.object({
  email: z.string().trim().min(1),
  inboundIds: z.array(z.number()).min(1),
  outboundTag: z.string().trim().min(1),
});

export type SubscriptionMixedNumber = z.infer<typeof SubscriptionMixedNumberSchema>;
export type SubscriptionInboundRef = z.infer<typeof SubscriptionInboundRefSchema>;
export type SubscriptionURL = z.infer<typeof SubscriptionUrlSchema>;
export type SubscriptionMember = z.infer<typeof SubscriptionMemberSchema>;
export type SubscriptionSummary = z.infer<typeof SubscriptionSummarySchema>;
export type SubscriptionDetail = z.infer<typeof SubscriptionDetailSchema>;
export type RoutedProfileRequest = z.infer<typeof RoutedProfileRequestSchema>;
