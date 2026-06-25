import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { HttpUtil, type Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  RoutedProfileRequestSchema,
  SubscriptionDetailSchema,
  SubscriptionSummaryListSchema,
  SubscriptionMemberSchema,
  type RoutedProfileRequest,
  type SubscriptionDetail,
  type SubscriptionMember,
  type SubscriptionSummary,
  type SubscriptionURL,
} from '@/schemas/subscription';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

async function fetchSubscriptions(): Promise<SubscriptionSummary[]> {
  const msg = await HttpUtil.get('/panel/api/subscriptions/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch subscriptions');
  const validated = parseMsg(msg, SubscriptionSummaryListSchema, 'subscriptions/list');
  return validated.obj ?? [];
}

async function fetchSubscriptionDetail(subId: string): Promise<SubscriptionDetail> {
  const msg = await HttpUtil.get(`/panel/api/subscriptions/get/${encodeURIComponent(subId)}`, undefined, { silent: true });
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to fetch subscription');
  const validated = parseMsg(msg, SubscriptionDetailSchema, 'subscriptions/get');
  if (!validated.obj) throw new Error('Empty subscription response');
  return validated.obj;
}

function invalidateSubscriptionData(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.subscriptions.root() }),
    queryClient.invalidateQueries({ queryKey: keys.clients.root() }),
    queryClient.invalidateQueries({ queryKey: keys.inbounds.root() }),
    queryClient.invalidateQueries({ queryKey: keys.xray.config() }),
  ]);
}

export function useSubscriptionsQuery() {
  const query = useQuery({
    queryKey: keys.subscriptions.list(),
    queryFn: fetchSubscriptions,
  });

  return {
    subscriptions: query.data ?? [],
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}

export function useSubscriptionDetailQuery(subId: string | null | undefined) {
  const cleanSubId = subId || '';
  const query = useQuery({
    queryKey: keys.subscriptions.detail(cleanSubId),
    queryFn: () => fetchSubscriptionDetail(cleanSubId),
    enabled: cleanSubId.length > 0,
  });

  return {
    detail: query.data ?? null,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}

export function useRoutedProfileMutation(subId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: RoutedProfileRequest): Promise<Msg<SubscriptionMember>> => {
      const body = RoutedProfileRequestSchema.parse(payload);
      const raw = await HttpUtil.post(
        `/panel/api/subscriptions/${encodeURIComponent(subId)}/routedProfile`,
        body,
        JSON_HEADERS,
      );
      return parseMsg(raw, SubscriptionMemberSchema, 'subscriptions/routedProfile');
    },
    onSuccess: (msg) => {
      if (msg?.success) void invalidateSubscriptionData(queryClient);
    },
  });
}

export type {
  RoutedProfileRequest,
  SubscriptionDetail,
  SubscriptionMember,
  SubscriptionSummary,
  SubscriptionURL,
};
