import { Box, Heading, Table, Tbody, Td, Th, Thead, Tr } from "@chakra-ui/react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { fmtUsd } from "@/lib/decimal";

export default function History() {
  const t = useQuery({ queryKey: ["trades"], queryFn: api.trades });
  const o = useQuery({ queryKey: ["orders"], queryFn: api.orders });
  return (
    <Box>
      <Heading size="md" mb={4}>交易记录</Heading>
      <Table size="sm" mb={8}>
        <Thead><Tr><Th>时间</Th><Th>市场</Th><Th>方向</Th><Th>价格</Th><Th>数量</Th><Th>手续费</Th></Tr></Thead>
        <Tbody>
          {(t.data || []).map((x) => (
            <Tr key={x.id}><Td>{x.created_at?.slice(11, 19)}</Td><Td>{x.symbol}</Td><Td>{x.side}</Td><Td>{fmtUsd(x.price)}</Td><Td>{x.qty}</Td><Td>{x.fee}</Td></Tr>
          ))}
        </Tbody>
      </Table>
      <Heading size="sm" mb={3}>订单</Heading>
      <Table size="sm">
        <Thead><Tr><Th>ID</Th><Th>市场</Th><Th>仓位</Th><Th>杠杆</Th><Th>状态</Th></Tr></Thead>
        <Tbody>
          {(o.data || []).map((x) => (
            <Tr key={x.id}><Td>{x.id}</Td><Td>{x.symbol}</Td><Td>{x.pos_side}</Td><Td>{x.leverage}x</Td><Td>{x.status}</Td></Tr>
          ))}
        </Tbody>
      </Table>
    </Box>
  );
}
