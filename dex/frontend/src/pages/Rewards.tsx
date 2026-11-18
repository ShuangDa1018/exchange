import { Box, Heading, SimpleGrid, Stat, StatHelpText, StatLabel, StatNumber, Text } from "@chakra-ui/react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { fmt } from "@/lib/decimal";

export default function Rewards() {
  const q = useQuery({ queryKey: ["points"], queryFn: api.points });
  return (
    <Box maxW="720px">
      <Heading size="md" mb={2}>积分活动</Heading>
      <Text color="gray.400" mb={6} fontSize="sm">成交额每 100 USDT = 1 积分。受邀注册额外赠送。</Text>
      <SimpleGrid columns={{ base: 1, md: 2 }} gap={4}>
        <Box bg="bg.800" p={6} borderRadius="md">
          <Stat><StatLabel>积分余额</StatLabel><StatNumber>{fmt(q.data?.balance || 0, 2)}</StatNumber><StatHelpText>可用于后续空投 / 费率折扣</StatHelpText></Stat>
        </Box>
        <Box bg="bg.800" p={6} borderRadius="md">
          <Stat><StatLabel>累计成交额</StatLabel><StatNumber>{fmt(q.data?.volume_usd || 0, 2)}</StatNumber><StatHelpText>USDT notional</StatHelpText></Stat>
        </Box>
      </SimpleGrid>
    </Box>
  );
}
