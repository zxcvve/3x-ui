import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Row,
  Select,
  Switch,
  message,
} from 'antd';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { RemoteInboundOption } from '@/api/queries/useNodeMutations';
import type { Msg } from '@/utils';
import {
  NodeFormSchema,
  NodeProvisionFormBaseSchema,
  NodeProvisionFormSchema,
  type NodeFormValues,
  type NodeProvisionFormValues,
  type NodeProvisionResult,
  type ProbeResult,
} from '@/schemas/node';
import { antdRule } from '@/utils/zodForm';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<Msg<ProbeResult>>;
  fetchFingerprint: (payload: Partial<NodeRecord>) => Promise<Msg<string>>;
  fetchInbounds: (payload: Partial<NodeRecord>) => Promise<Msg<RemoteInboundOption[]>>;
  save: (payload: Partial<NodeRecord>) => Promise<Msg<unknown>>;
  provision: (payload: NodeProvisionFormValues) => Promise<Msg<NodeProvisionResult>>;
  onOpenChange: (open: boolean) => void;
}

function defaultValues(): NodeFormValues {
  return {
    id: 0,
    name: '',
    remark: '',
    scheme: 'https',
    address: '',
    port: 2053,
    basePath: '/',
    apiToken: '',
    enable: true,
    allowPrivateAddress: false,
    tlsVerifyMode: 'verify',
    pinnedCertSha256: '',
    inboundSyncMode: 'all',
    inboundTags: [],
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  testConnection,
  fetchFingerprint,
  fetchInbounds,
  save,
  provision,
  onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<NodeFormValues & NodeProvisionFormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [addMode, setAddMode] = useState<'existing' | 'provision'>('existing');
  const [testing, setTesting] = useState(false);
  const [fetchingPin, setFetchingPin] = useState(false);
  const [fetchingInbounds, setFetchingInbounds] = useState(false);
  const [inboundOptions, setInboundOptions] = useState<RemoteInboundOption[]>([]);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);
  const [provisionResult, setProvisionResult] = useState<NodeProvisionResult | null>(null);
  const scheme = Form.useWatch('scheme', form) ?? 'https';
  const tlsVerifyMode = Form.useWatch('tlsVerifyMode', form) ?? 'verify';
  const inboundSyncMode = Form.useWatch('inboundSyncMode', form) ?? 'all';
  const sslMode = Form.useWatch('sslMode', form) ?? 'none';
  const isProvision = mode === 'add' && addMode === 'provision';

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<NodeFormValues>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
        inboundSyncMode: (node.inboundSyncMode as 'all' | 'selected') || base.inboundSyncMode,
        inboundTags: node.inboundTags ?? [],
      }
      : {
        ...base,
        sshHost: '',
        sshPort: 22,
        sshUser: 'root',
        sshPassword: '',
        sshPrivateKey: '',
        sshPrivateKeyPass: '',
        sshHostKeySha256: '',
        sudoPassword: '',
        panelPort: undefined,
        webBasePath: '',
        sslMode: 'none',
        domain: '',
        acmeEmail: '',
      };
    if (next.scheme === 'http') next.tlsVerifyMode = 'skip';
    if (mode === 'edit') setAddMode('existing');
    form.resetFields();
    form.setFieldsValue(next);
    setInboundOptions((next.inboundTags || []).map((tag) => ({ tag })));
    setTestResult(null);
    setProvisionResult(null);
  }, [open, mode, node, form]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(values: NodeFormValues): Partial<NodeRecord> {
    return {
      id: values.id || 0,
      name: values.name.trim(),
      remark: values.remark?.trim() || '',
      scheme: values.scheme,
      address: values.address.trim(),
      port: values.port,
      basePath: values.basePath.trim() || '/',
      apiToken: values.apiToken.trim(),
      enable: values.enable,
      allowPrivateAddress: values.allowPrivateAddress,
      tlsVerifyMode: values.tlsVerifyMode,
      pinnedCertSha256: values.tlsVerifyMode === 'pin' ? values.pinnedCertSha256.trim() : '',
      inboundSyncMode: values.inboundSyncMode,
      inboundTags: values.inboundSyncMode === 'selected' ? values.inboundTags : [],
    };
  }

  async function onTest() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await testConnection(payload);
      if (msg?.success && msg.obj) {
        setTestResult(msg.obj);
      } else {
        setTestResult({ status: 'offline', error: msg?.msg || 'unknown error' });
      }
    } finally {
      setTesting(false);
    }
  }

  async function onFetchPin() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setFetchingPin(true);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await fetchFingerprint(payload);
      if (msg?.success && msg.obj) {
        form.setFieldValue('pinnedCertSha256', msg.obj);
        messageApi.success(t('pages.nodes.pinFetched'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.pinFetchFailed'));
      }
    } finally {
      setFetchingPin(false);
    }
  }

  async function onFetchInbounds() {
    try {
      await form.validateFields(['name', 'address', 'port', 'apiToken']);
    } catch {
      return;
    }
    setFetchingInbounds(true);
    try {
      const msg = await fetchInbounds(buildPayload(form.getFieldsValue(true)));
      if (msg?.success && Array.isArray(msg.obj)) {
        setInboundOptions(msg.obj);
        messageApi.success(t('pages.nodes.inboundsLoaded', { count: msg.obj.length }));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.inboundsLoadFailed'));
      }
    } finally {
      setFetchingInbounds(false);
    }
  }

  async function onFinish(values: NodeFormValues) {
    if (isProvision) {
      const result = NodeProvisionFormSchema.safeParse(values);
      if (!result.success) {
        messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
        return;
      }
      setSubmitting(true);
      setProvisionResult(null);
      try {
        const msg = await provision(result.data);
        if (msg?.success) {
          setProvisionResult(msg.obj ?? null);
          onOpenChange(false);
        } else {
          setProvisionResult(msg?.obj ?? null);
          messageApi.error(msg?.msg || t('pages.nodes.toasts.provisionFailed'));
        }
      } finally {
        setSubmitting(false);
      }
      return;
    }

    const result = NodeFormSchema.safeParse(values);
    if (!result.success) {
      messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
      return;
    }
    setSubmitting(true);
    try {
      const payload = buildPayload(result.data);
      const test = await testConnection(payload);
      const probe = test?.success ? test.obj : null;
      if (!probe || probe.status !== 'online') {
        setTestResult(probe ?? { status: 'offline', error: test?.msg || t('pages.nodes.connectionFailed') });
        return;
      }
      setTestResult(probe);
      const msg = await save(payload);
      if (msg?.success) {
        onOpenChange(false);
      }
    } finally {
      setSubmitting(false);
    }
  }

  function close() {
    if (!submitting) onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting}
        okText={isProvision ? t('pages.nodes.provision') : t('save')}
        cancelText={t('cancel')}
        mask={{ closable: false }}
        width="640px"
        onOk={() => form.submit()}
        onCancel={close}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={defaultValues()}
          onFinish={onFinish}
        >
          {mode === 'add' && (
            <Form.Item label={t('pages.nodes.addMode')}>
              <Radio.Group
                value={addMode}
                onChange={(e) => {
                  setAddMode(e.target.value);
                  setTestResult(null);
                  setProvisionResult(null);
                }}
              >
                <Radio.Button value="existing">{t('pages.nodes.existingPanel')}</Radio.Button>
                <Radio.Button value="provision">{t('pages.nodes.provisionViaSsh')}</Radio.Button>
              </Radio.Group>
            </Form.Item>
          )}

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.name')}
                name="name"
                rules={[antdRule(NodeFormSchema.shape.name, t)]}
              >
                <Input placeholder={t('pages.nodes.namePlaceholder')} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.remark')} name="remark">
                <Input />
              </Form.Item>
            </Col>
          </Row>

          {isProvision ? (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message={t('pages.nodes.provisionHint')}
              />

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshHost')}
                    name="sshHost"
                    rules={[antdRule(NodeProvisionFormBaseSchema.shape.sshHost, t)]}
                  >
                    <Input placeholder="203.0.113.10" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item label={t('pages.nodes.sshPort')} name="sshPort">
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item
                    label={t('pages.nodes.sshUser')}
                    name="sshUser"
                    rules={[antdRule(NodeProvisionFormBaseSchema.shape.sshUser, t)]}
                  >
                    <Input placeholder="root" />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                label={t('pages.nodes.sshHostKeySha256')}
                name="sshHostKeySha256"
                extra={t('pages.nodes.sshHostKeyHint')}
                rules={[antdRule(NodeProvisionFormBaseSchema.shape.sshHostKeySha256, t)]}
              >
                <Input placeholder="SHA256:..." />
              </Form.Item>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item label={t('pages.nodes.sshPassword')} name="sshPassword">
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item label={t('pages.nodes.sudoPassword')} name="sudoPassword" extra={t('pages.nodes.sudoPasswordHint')}>
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item label={t('pages.nodes.sshPrivateKey')} name="sshPrivateKey">
                <Input.TextArea rows={4} autoComplete="off" />
              </Form.Item>

              <Form.Item label={t('pages.nodes.sshPrivateKeyPass')} name="sshPrivateKeyPass">
                <Input.Password autoComplete="new-password" />
              </Form.Item>

              <Row gutter={16}>
                <Col xs={24} md={8}>
                  <Form.Item label={t('pages.nodes.sslMode')} name="sslMode">
                    <Select
                      options={[
                        { value: 'none', label: 'none' },
                        { value: 'ip', label: 'ip' },
                        { value: 'domain', label: 'domain' },
                      ]}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item label={t('pages.nodes.panelPort')} name="panelPort">
                    <InputNumber min={1} max={65535} placeholder={t('pages.nodes.random')} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item label={t('pages.nodes.webBasePath')} name="webBasePath">
                    <Input placeholder={t('pages.nodes.random')} />
                  </Form.Item>
                </Col>
              </Row>

              {sslMode === 'domain' && (
                <Row gutter={16}>
                  <Col xs={24} md={12}>
                    <Form.Item label={t('pages.nodes.domain')} name="domain">
                      <Input placeholder="panel.example.com" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label={t('pages.nodes.acmeEmail')} name="acmeEmail">
                      <Input placeholder="admin@example.com" />
                    </Form.Item>
                  </Col>
                </Row>
              )}

              <Form.Item
                label={t('pages.nodes.allowPrivateAddress')}
                name="allowPrivateAddress"
                valuePropName="checked"
                extra={t('pages.nodes.allowPrivateAddressHint')}
              >
                <Switch />
              </Form.Item>

              {provisionResult?.output && provisionResult.output.length > 0 && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message={t('pages.nodes.provisionOutput')}
                  description={<pre className="provision-output">{provisionResult.output.join('\n')}</pre>}
                />
              )}
            </>
          ) : (
            <>
          <Row gutter={16}>
            <Col xs={24} md={6}>
              <Form.Item label={t('pages.nodes.scheme')} name="scheme">
                <Select
                  options={[
                    { value: 'https', label: 'https' },
                    { value: 'http', label: 'http' },
                  ]}
                  onChange={(value) => {
                    if (value === 'http') form.setFieldValue('tlsVerifyMode', 'skip');
                  }}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.address')}
                name="address"
                rules={[antdRule(NodeFormSchema.shape.address, t)]}
              >
                <Input placeholder={t('pages.nodes.addressPlaceholder')} />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item
                label={t('pages.nodes.port')}
                name="port"
                rules={[antdRule(NodeFormSchema.shape.port, t)]}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.basePath')} name="basePath">
                <Input placeholder="/" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.enable')}
                name="enable"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            label={t('pages.nodes.allowPrivateAddress')}
            name="allowPrivateAddress"
            valuePropName="checked"
            extra={t('pages.nodes.allowPrivateAddressHint')}
          >
            <Switch />
          </Form.Item>

          <Form.Item
            label={t('pages.nodes.tlsVerifyMode')}
            name="tlsVerifyMode"
            extra={t('pages.nodes.tlsVerifyModeHint')}
          >
            <Select
              disabled={scheme === 'http'}
              options={[
                { value: 'verify', label: t('pages.nodes.tlsVerify') },
                { value: 'pin', label: t('pages.nodes.tlsPin') },
                { value: 'skip', label: t('pages.nodes.tlsSkip') },
              ]}
            />
          </Form.Item>

          {tlsVerifyMode === 'skip' && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              title={t('pages.nodes.tlsSkipWarning')}
            />
          )}

          {tlsVerifyMode === 'pin' && (
            <Form.Item
              label={t('pages.nodes.pinnedCert')}
              name="pinnedCertSha256"
              extra={t('pages.nodes.pinnedCertHint')}
            >
              <Input.Search
                placeholder={t('pages.nodes.pinnedCertPlaceholder')}
                enterButton={t('pages.nodes.fetchPin')}
                loading={fetchingPin}
                onSearch={onFetchPin}
              />
            </Form.Item>
          )}

          <Form.Item
            label={t('pages.nodes.apiToken')}
            name="apiToken"
            rules={[antdRule(NodeFormSchema.shape.apiToken, t)]}
            extra={t('pages.nodes.apiTokenHint')}
          >
            <Input.Password placeholder={t('pages.nodes.apiTokenPlaceholder')} />
          </Form.Item>

          <Form.Item
            label={t('pages.nodes.inboundSyncMode')}
            name="inboundSyncMode"
            extra={t('pages.nodes.inboundSyncModeHint')}
          >
            <Select
              options={[
                { value: 'all', label: t('pages.nodes.allInbounds') },
                { value: 'selected', label: t('pages.nodes.selectedInbounds') },
              ]}
            />
          </Form.Item>

          {inboundSyncMode === 'selected' && (
            <Form.Item
              label={t('pages.nodes.inboundTags')}
              name="inboundTags"
              extra={t('pages.nodes.inboundTagsHint')}
            >
              <Select
                mode="multiple"
                allowClear
                loading={fetchingInbounds}
                placeholder={t('pages.nodes.inboundTagsPlaceholder')}
                popupRender={(menu) => (
                  <>
                    <Button type="text" block loading={fetchingInbounds} onClick={onFetchInbounds}>
                      {t('pages.nodes.loadInbounds')}
                    </Button>
                    {menu}
                  </>
                )}
                options={inboundOptions.map((inbound) => ({
                  value: inbound.tag,
                  label: `${inbound.remark || inbound.tag}${inbound.protocol ? ` (${inbound.protocol}:${inbound.port || 0})` : ''}`,
                }))}
              />
            </Form.Item>
          )}

          <div className="test-row">
            <Button type="default" loading={testing} onClick={onTest}>
              {t('pages.nodes.testConnection')}
            </Button>
            {testResult && (
              <div className="test-result">
                {testResult.status === 'online' ? (
                  <Alert
                    type="success"
                    showIcon
                    title={t('pages.nodes.connectionOk', { ms: testResult.latencyMs })}
                    description={testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined}
                  />
                ) : (
                  <Alert
                    type="error"
                    showIcon
                    title={t('pages.nodes.connectionFailed')}
                    description={testResult.error}
                  />
                )}
              </div>
            )}
          </div>
            </>
          )}
        </Form>
      </Modal>
    </>
  );
}
