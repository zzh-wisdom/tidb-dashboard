import React from 'react'
import { Table, Card, Skeleton } from 'antd'
import { useTranslation } from 'react-i18next'

function ComponentPanelTable({ cluster }) {
  const { t } = useTranslation()
  const columns = [
    {
      title: t('cluster_info.component_table.address'),
      dataIndex: 'address',
      key: 'address',
      ellipsis: true,
      width: 240,
    },
    {
      title: t('cluster_info.component_table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
    },
    {
      title: t('cluster_info.component_table.version'),
      dataIndex: 'version',
      key: 'version',
      ellipsis: true,
      width: 200,
    },
    {
      title: t('cluster_info.component_table.deploy_path'),
      dataIndex: 'deploy_path',
      key: 'deploy_path',
      ellipsis: true,
    },
  ]

  let dataSource = []
  if (cluster) {
    pushNodes('tikv', cluster, dataSource)
    pushNodes('tidb', cluster, dataSource)
    pushNodes('pd', cluster, dataSource)
  }

  return (
    <Card
      size="small"
      bordered={false}
      title={t('cluster_info.component_table.node_list')}
    >
      {!cluster ? (
        <Skeleton active title={false} paragraph={{ rows: 5 }} />
      ) : (
        <Table
          columns={columns}
          dataSource={dataSource}
          pagination={false}
          size="middle"
          style={{ margin: -5 }}
        />
      )}
    </Card>
  )
}

function pushNodes(key, cluster, dataSource) {
  if (
    cluster[key] !== undefined &&
    cluster[key] !== null &&
    cluster[key].err === null
  ) {
    const nodes = cluster[key].nodes
    dataSource.push({
      address: key + '(' + nodes.length + ')',
      children: nodes.map((n, index) => wrapNode(n, key, index)),
    })
  }
}

function wrapNode(node, comp, id) {
  if (node === undefined || node === null) {
    return
  }
  let status = 'down'
  if (node.status === 1) {
    status = 'up'
  }
  if (node.deploy_path === undefined && node.binary_path !== null) {
    node.deploy_path = node.binary_path.substring(
      0,
      node.binary_path.lastIndexOf('/')
    )
  }
  return {
    address: `${node.ip}:${node.port}`,
    binary_path: node.binary_path,
    deploy_path: node.deploy_path,
    version: node.version,
    status_port: node.status_port,
    status: status,
  }
}

export default ComponentPanelTable
