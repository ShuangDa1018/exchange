import { Box, Button, Flex, Heading, Input, SimpleGrid, Stat, StatLabel, StatNumber, Text, useToast } from "@chakra-ui/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { api } from "@/api/client";
import { fmt } from "@/lib/decimal";

export default function Assets() {
  const toast = useToast();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["assets"], queryFn: api.assets });
  const led = useQuery({ queryKey: ["ledger"], queryFn: api.ledger });
  const [amt, setAmt] = useState("1000");
  const [addr, setAddr] = useState("0xTFDEX000000000000000000000000000000");

  return (
    <Box maxW="900px">
      <Heading size="md" mb={4}>资产 · 充提</Heading>
      <SimpleGrid columns={{ base: 1, md: 3 }} gap={3} mb={6}>
        {(q.data || []).map((b) => (
          <Box key={b.asset} bg="bg.800" p={4} borderRadius="md">
            <Stat><StatLabel>{b.asset}</StatLabel><StatNumber>{fmt(b.available, 4)}</StatNumber></Stat>
            <Text fontSize="xs" color="gray.500">冻结 {fmt(b.locked, 4)}</Text>
          </Box>
        ))}
      </SimpleGrid>
      <Flex gap={6} direction={{ base: "column", md: "row" }} mb={8}>
        <Box flex="1" bg="bg.800" p={4} borderRadius="md">
          <Text mb={2}>模拟充值 USDT</Text>
          <Input value={amt} onChange={(e) => setAmt(e.target.value)} mb={3} />
          <Button colorScheme="teal" onClick={async () => { await api.deposit("USDT", amt); toast({ title: "充值成功", status: "success" }); void qc.invalidateQueries(); }}>充值</Button>
        </Box>
        <Box flex="1" bg="bg.800" p={4} borderRadius="md">
          <Text mb={2}>提现地址（演示）</Text>
          <Input value={addr} onChange={(e) => setAddr(e.target.value)} mb={3} />
          <Box bg="white" p={2} w="fit-content" mb={3}><QRCodeSVG value={addr} size={120} /></Box>
          <Button variant="outline" onClick={async () => { await api.withdraw("USDT", amt, addr); toast({ title: "已广播提现", status: "info" }); void qc.invalidateQueries(); }}>提现 {amt} USDT</Button>
        </Box>
      </Flex>
      <Heading size="sm" mb={2}>资金流水</Heading>
      {(led.data || []).slice(0, 20).map((x: any) => (
        <Flex key={x.id} justify="space-between" fontSize="sm" py={1} color="gray.300">
          <span>{x.kind}</span><span>{x.asset} {x.amount}</span>
        </Flex>
      ))}
    </Box>
  );
}
