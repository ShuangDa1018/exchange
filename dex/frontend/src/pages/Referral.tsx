import { Box, Button, Heading, Stat, StatLabel, StatNumber, Text, useToast } from "@chakra-ui/react";
import { useQuery } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { api } from "@/api/client";

export default function Referral() {
  const q = useQuery({ queryKey: ["referral"], queryFn: api.referral });
  const toast = useToast();
  const link = `${location.origin}/login?invite=${q.data?.code || ""}`;
  return (
    <Box maxW="640px">
      <Heading size="md" mb={2}>推荐返佣</Heading>
      <Text color="gray.400" mb={6} fontSize="sm">好友手续费 30% 返佣即时到账 USDT，双方各得积分。</Text>
      <Box bg="bg.800" p={6} borderRadius="md" mb={4}>
        <Stat mb={4}><StatLabel>邀请码</StatLabel><StatNumber>{q.data?.code}</StatNumber></Stat>
        <Box bg="white" p={2} w="fit-content" mb={4}><QRCodeSVG value={link} size={140} /></Box>
        <Button onClick={() => { navigator.clipboard.writeText(link); toast({ title: "已复制邀请链接", status: "success" }); }}>复制链接</Button>
      </Box>
      <Text>邀请人数 {q.data?.invitees || 0} · 返佣 {q.data?.rebate_usdt || 0} USDT · 积分 {q.data?.points || 0}</Text>
    </Box>
  );
}
