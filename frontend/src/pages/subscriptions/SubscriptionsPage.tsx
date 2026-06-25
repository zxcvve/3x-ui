import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
  Layout,
  Modal,
  QRCode,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  CopyOutlined,
  LinkOutlined,
  PlusOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { useOutboundTags } from '@/api/queries/useOutboundTags';
import {
  useRoutedProfileMutation,
  useSubscriptionDetailQuery,
  useSubscriptionsQuery,
  type RoutedProfileRequest,
  type SubscriptionMember,
  type SubscriptionSummary,
  type SubscriptionURL,
} from '@/api/queries/useSubscriptionsQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useTheme } from '@/hooks/useTheme';
import { ClipboardManager, SizeFormatter } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import './SubscriptionsPage.css';

function usageText(up = 0, down = 0, total = 0) {
  const used = up + down;
  if (total > 0) return `${SizeFormatter.sizeFormat(used)} / ${SizeFormatter.sizeFormat(total)}`;
  return SizeFormatter.sizeFormat(used);
}

function mixedText(value: { value: number; mixed: boolean } | undefined, format: (n: number) => string) {
  if (!value) return '-';
  if (value.mixed) return 'Mixed';
  if (!value.value) return 'Unlimited';
  return format(value.value);
}

function dateText(ms = 0) {
  if (!ms) return 'Unlimited';
  return new Date(ms).toLocaleString();
}

function inboundTags(inbounds: Array<{ id: number; tag?: string; remark?: string }> = []) {
  if (inbounds.length === 0) return <Typography.Text type="secondary">-</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {inbounds.map((ib) => (
        <Tag key={ib.id}>{ib.remark || ib.tag || ib.id}</Tag>
      ))}
    </Space>
  );
}

function SubscriptionUrlActions({ urls }: { urls: SubscriptionURL[] }) {
  const [qr, setQr] = useState<SubscriptionURL | null>(null);
  async function copy(url: string) {
    const ok = await ClipboardManager.copyText(url);
    if (ok) message.success('Copied');
  }

  return (
    <>
      <Space orientation="vertical" className="subscription-url-list">
        {urls.length === 0 ? (
          <Typography.Text type="secondary">No enabled subscription URLs</Typography.Text>
        ) : urls.map((item) => (
          <div className="subscription-url-row" key={item.format}>
            <Tag color={item.format === 'raw' ? 'blue' : item.format === 'json' ? 'green' : 'purple'}>{item.format}</Tag>
            <Typography.Text ellipsis className="subscription-url-text">{item.url}</Typography.Text>
            <Tooltip title="Copy">
              <Button size="small" icon={<CopyOutlined />} onClick={() => copy(item.url)} />
            </Tooltip>
            <Tooltip title="QR">
              <Button size="small" icon={<QrcodeOutlined />} onClick={() => setQr(item)} />
            </Tooltip>
          </div>
        ))}
      </Space>
      <Modal open={!!qr} onCancel={() => setQr(null)} footer={null} title={qr?.format.toUpperCase()} width={360} destroyOnHidden>
        {qr && (
          <div className="subscription-qr">
            <QRCode value={qr.url} size={220} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
            <Typography.Text copyable={{ text: qr.url }} className="subscription-qr-url">{qr.url}</Typography.Text>
          </div>
        )}
      </Modal>
    </>
  );
}

function RoutedProfileModal({
  open,
  subId,
  inboundOptions,
  outboundTags,
  onCancel,
  onCreated,
}: {
  open: boolean;
  subId: string;
  inboundOptions: Array<{ id: number; remark?: string; tag?: string }>;
  outboundTags: string[];
  onCancel: () => void;
  onCreated: () => void;
}) {
  const [form] = Form.useForm<RoutedProfileRequest>();
  const mutation = useRoutedProfileMutation(subId);

  useEffect(() => {
    if (open) form.resetFields();
  }, [open, form]);

  async function submit() {
    const values = await form.validateFields();
    const msg = await mutation.mutateAsync(values);
    if (msg?.success) {
      message.success('Profile created');
      onCreated();
    } else if (msg?.msg) {
      message.error(msg.msg);
    }
  }

  return (
    <Modal
      open={open}
      title="Routed profile"
      onCancel={onCancel}
      onOk={submit}
      okButtonProps={{ loading: mutation.isPending }}
      destroyOnHidden
    >
      <Form form={form} layout="vertical">
        <Form.Item name="email" label="Email" rules={[{ required: true }]}>
          <Input autoComplete="off" />
        </Form.Item>
        <Form.Item name="inboundIds" label="Inbounds" rules={[{ required: true }]}>
          <Select
            mode="multiple"
            options={inboundOptions.map((ib) => ({ value: ib.id, label: ib.remark || ib.tag || String(ib.id) }))}
          />
        </Form.Item>
        <Form.Item name="outboundTag" label="Outbound" rules={[{ required: true }]}>
          <Select showSearch options={outboundTags.map((tag) => ({ value: tag, label: tag }))} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export default function SubscriptionsPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { subscriptions, loading, fetched, fetchError, refetch } = useSubscriptionsQuery();
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState<'all' | 'enabled' | 'disabled' | 'mixed'>('all');
  const [selectedSubId, setSelectedSubId] = useState('');
  const selected = selectedSubId || subscriptions[0]?.subId || '';
  const { detail, loading: detailLoading, refetch: refetchDetail } = useSubscriptionDetailQuery(selected);
  const { data: inboundOptions = [] } = useInboundOptions();
  const { data: outboundTags = [] } = useOutboundTags();
  const [profileOpen, setProfileOpen] = useState(false);

  useEffect(() => {
    if (!selectedSubId && subscriptions[0]?.subId) setSelectedSubId(subscriptions[0].subId);
  }, [subscriptions, selectedSubId]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return subscriptions.filter((s) => {
      if (needle && !s.subId.toLowerCase().includes(needle)) return false;
      if (stateFilter === 'enabled') return s.enabledCount > 0 && s.disabledCount === 0;
      if (stateFilter === 'disabled') return s.enabledCount === 0;
      if (stateFilter === 'mixed') return s.enabledCount > 0 && s.disabledCount > 0;
      return true;
    });
  }, [subscriptions, search, stateFilter]);

  const summary = useMemo(() => ({
    total: subscriptions.length,
    members: subscriptions.reduce((acc, s) => acc + s.memberCount, 0),
    enabled: subscriptions.reduce((acc, s) => acc + s.enabledCount, 0),
  }), [subscriptions]);

  const listColumns = useMemo<TableColumnsType<SubscriptionSummary>>(() => [
    {
      title: 'Subscription',
      dataIndex: 'subId',
      render: (subId: string, row) => (
        <Button type="link" className="subscription-id-link" onClick={() => setSelectedSubId(subId)}>
          <Badge status={row.disabledCount === 0 ? 'success' : row.enabledCount === 0 ? 'default' : 'warning'} />
          <span>{subId}</span>
        </Button>
      ),
    },
    { title: 'Members', dataIndex: 'memberCount', width: 96 },
    {
      title: 'Usage',
      width: 180,
      render: (_, row) => usageText(row.traffic?.up, row.traffic?.down, row.traffic?.total),
      responsive: ['md'],
    },
    {
      title: 'Inbounds',
      width: 220,
      render: (_, row) => inboundTags(row.inbounds),
      responsive: ['lg'],
    },
  ], []);

  const memberColumns = useMemo<TableColumnsType<SubscriptionMember>>(() => [
    {
      title: 'Email',
      dataIndex: 'email',
      render: (email: string, row) => (
        <Space size={6} wrap>
          <Badge status={row.enable ? 'success' : 'default'} />
          <Typography.Text>{email}</Typography.Text>
          {row.routeTag && <Tag color="green">{row.routeTag}</Tag>}
        </Space>
      ),
    },
    {
      title: 'Usage',
      width: 180,
      render: (_, row) => usageText(row.traffic?.up, row.traffic?.down, row.traffic?.total),
    },
    {
      title: 'Expiry',
      width: 180,
      render: (_, row) => dateText(row.expiryTime),
      responsive: ['md'],
    },
    {
      title: 'Inbounds',
      width: 240,
      render: (_, row) => inboundTags(row.inbounds),
      responsive: ['lg'],
    },
  ], []);

  const pageClass = ['subscriptions-page', isDark ? 'is-dark' : '', isUltra ? 'is-ultra' : ''].filter(Boolean).join(' ');

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" loading={loading} onClick={() => refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" className="summary-card">
                      <Row gutter={[12, 8]}>
                        <Col xs={8}><Statistic title="Subscriptions" value={summary.total} prefix={<LinkOutlined />} /></Col>
                        <Col xs={8}><Statistic title="Profiles" value={summary.members} prefix={<TeamOutlined />} /></Col>
                        <Col xs={8}><Statistic title="Enabled" value={summary.enabled} /></Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col xs={24} lg={10} xl={9}>
                    <Card
                      size="small"
                      title="Subscriptions"
                      extra={<Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={() => refetch()} />}
                    >
                      <Space className="subscription-toolbar" wrap>
                        <Input.Search value={search} onChange={(e) => setSearch(e.target.value)} allowClear placeholder="Search subId" />
                        <Select
                          value={stateFilter}
                          onChange={setStateFilter}
                          className="subscription-state-filter"
                          options={[
                            { value: 'all', label: 'All' },
                            { value: 'enabled', label: 'Enabled' },
                            { value: 'disabled', label: 'Disabled' },
                            { value: 'mixed', label: 'Mixed' },
                          ]}
                        />
                      </Space>
                      <Table
                        rowKey="subId"
                        size="small"
                        columns={listColumns}
                        dataSource={filtered}
                        pagination={{ pageSize: 10, size: 'small' }}
                        rowClassName={(row) => (row.subId === selected ? 'subscription-row-selected' : '')}
                        onRow={(row) => ({ onClick: () => setSelectedSubId(row.subId) })}
                        scroll={{ x: 520 }}
                      />
                    </Card>
                  </Col>

                  <Col xs={24} lg={14} xl={15}>
                    <Card
                      size="small"
                      title={selected || 'Subscription'}
                      extra={selected && (
                        <Space>
                          <Button size="small" icon={<ReloadOutlined />} loading={detailLoading} onClick={() => refetchDetail()} />
                          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setProfileOpen(true)}>Profile</Button>
                        </Space>
                      )}
                    >
                      {!selected ? (
                        <Result status="info" title="No subscriptions" />
                      ) : (
                        <Spin spinning={detailLoading}>
                          <Row gutter={[12, 12]}>
                            <Col span={24}>
                              <Space size={[6, 6]} wrap>
                                <Tag>{detail?.memberCount ?? 0} profiles</Tag>
                                <Tag color={(detail?.disabledCount ?? 0) === 0 ? 'green' : 'orange'}>
                                  {detail?.enabledCount ?? 0} enabled / {detail?.disabledCount ?? 0} disabled
                                </Tag>
                                <Tag>{mixedText(detail?.totalGB, SizeFormatter.sizeFormat)}</Tag>
                                <Tag>{mixedText(detail?.expiryTime, dateText)}</Tag>
                              </Space>
                            </Col>
                            <Col span={24}>
                              <SubscriptionUrlActions urls={detail?.urls ?? []} />
                            </Col>
                            <Col span={24}>
                              <Table
                                rowKey="email"
                                size="small"
                                columns={memberColumns}
                                dataSource={detail?.members ?? []}
                                pagination={false}
                                scroll={{ x: 720 }}
                              />
                            </Col>
                          </Row>
                        </Spin>
                      )}
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <RoutedProfileModal
          open={profileOpen}
          subId={selected}
          inboundOptions={inboundOptions}
          outboundTags={outboundTags}
          onCancel={() => setProfileOpen(false)}
          onCreated={() => {
            setProfileOpen(false);
            void refetchDetail();
          }}
        />
      </Layout>
    </ConfigProvider>
  );
}
