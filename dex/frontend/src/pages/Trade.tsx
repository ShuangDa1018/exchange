import { Box, Button, Flex, Grid, HStack, Input, Select, Stat, StatHelpText, StatLabel, StatNumber, Tab, TabList, TabPanel, TabPanels, Tabs, Text } from "@chakra-ui/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { TVChart } from "@/components/TVChart";
import { api, type Market } from "@/api/client";
import { dexSocket } from "@/api/ws";
import { datafeed } from "@/lib/datafeed";
import { fmt, fmtUsd, liqPrice, marginOf, unrealizedPnl } from "@/lib/decimal";
import { useSelector } from "react-redux";
import type { RootState } from "@/store";

export default function Trade() {
  const token = useSelector((s: RootState) => s.session.token);
  const qc = useQueryClient();
  const [symbol, setSymbol] = useState("BTC-USDT");
  const [side, setSide] = useState<"buy" | "sell">("buy");
  const [lev, setLev] = useState(10);
  const [qty, setQty] = useState("0.01");
  const marketsQ = useQuery({ queryKey: ["markets"], queryFn: api.markets, refetchInterval: 4000 });
  const bookQ = useQuery({ queryKey: ["book", symbol], queryFn: () => api.book(symbol) });
  const posQ = useQuery({ queryKey: ["positions"], queryFn: api.positions, enabled: !!token });
  const ordQ = useQuery({ queryKey: ["orders"], queryFn: api.orders, enabled: !!token });
  const balQ = useQuery({ queryKey: ["assets"], queryFn: api.assets, enabled: !!token });
  const tradesQ = useQuery({ queryKey: ["ptrades", symbol], queryFn: () => api.publicTrades(symbol) });

  const market = useMemo(() => marketsQ.data?.find((m) => m.symbol === symbol) || marketsQ.data?.[0], [marketsQ.data, symbol]);

  useEffect(() => {
    dexSocket.subscribe("ticker", symbol);
    dexSocket.subscribe("orderbook", symbol);
    dexSocket.subscribe("kline", symbol);
    const off = dexSocket.on((msg) => {
      if (msg.channel === "kline" && msg.symbol === symbol) datafeed.applyKline(symbol, msg.interval || "1m", msg.data);
      if (msg.channel === "orderbook" && msg.symbol === symbol) qc.setQueryData(["book", symbol], msg.data);
      if (msg.channel === "ticker") qc.setQueryData(["markets"], (old?: Market[]) => {
        if (!old) return old;
        return old.map((m) => m.symbol === msg.symbol ? { ...m, price: msg.data.price, mark_price: msg.data.mark, volume_24h: msg.data.volume } : m);
      });
    });
    return () => {
      dexSocket.unsubscribe("ticker", symbol);
      dexSocket.unsubscribe("orderbook", symbol);
      dexSocket.unsubscribe("kline", symbol);
      off();
    };
  }, [symbol, qc]);

  const place = useMutation({
    mutationFn: () => api.place({ symbol, side, pos_side: side === "buy" ? "long" : "short", type: "market", qty, leverage: lev }),
    onSuccess: () => { void qc.invalidateQueries(); },
  });
  const close = useMutation({
    mutationFn: (id: string) => api.closePos(id),
    onSuccess: () => { void qc.invalidateQueries(); },
  });

  const mark = market?.mark_price || market?.price || "0";
  const usdt = balQ.data?.find((b) => b.asset === "USDT");
  const mgn = marginOf(mark, qty, lev).toFixed(4);

  return (
    <Grid templateColumns={{ base: "1fr", xl: "1fr 320px" }} gap={3}>
      <Box>
        <HStack mb={2} overflowX="auto" spacing={2}>
          {(marketsQ.data || []).map((m) => (
            <Button key={m.symbol} size="xs" variant={m.symbol === symbol ? "solid" : "ghost"} onClick={() => setSymbol(m.symbol)}>
              {m.base} {Number(m.change_24h) >= 0 ? "+" : ""}{(Number(m.change_24h) * 100).toFixed(2)}%
            </Button>
          ))}
        </HStack>
        <Flex gap={6} mb={2} wrap="wrap">
          <Stat size="sm"><StatLabel>{symbol}</StatLabel><StatNumber>{fmtUsd(mark)}</StatNumber><StatHelpText>Mark</StatHelpText></Stat>
          <Stat size="sm"><StatLabel>24h High</StatLabel><StatNumber fontSize="md">{fmtUsd(market?.high_24h || 0)}</StatNumber></Stat>
          <Stat size="sm"><StatLabel>24h Low</StatLabel><StatNumber fontSize="md">{fmtUsd(market?.low_24h || 0)}</StatNumber></Stat>
          <Stat size="sm"><StatLabel>Funding</StatLabel><StatNumber fontSize="md">{market?.funding_rate}</StatNumber></Stat>
        </Flex>
        <TVChart symbol={symbol} />
      </Box>

      <Box bg="bg.800" p={3} borderRadius="md" borderWidth="1px" borderColor="whiteAlpha.100">
        <HStack mb={3}>
          <Button flex="1" bg={side === "buy" ? "up.500" : "whiteAlpha.100"} color={side === "buy" ? "black" : "white"} onClick={() => setSide("buy")}>开多</Button>
          <Button flex="1" bg={side === "sell" ? "down.500" : "whiteAlpha.100"} onClick={() => setSide("sell")}>开空</Button>
        </HStack>
        <Text fontSize="xs" color="gray.500" mb={1}>杠杆 {lev}x</Text>
        <Select size="sm" value={lev} onChange={(e) => setLev(+e.target.value)} mb={3}>
          {[1, 2, 5, 10, 20, 50].map((n) => <option key={n} value={n}>{n}x</option>)}
        </Select>
        <Text fontSize="xs" color="gray.500" mb={1}>数量 ({market?.base})</Text>
        <Input size="sm" value={qty} onChange={(e) => setQty(e.target.value)} mb={3} />
        <Text fontSize="xs" color="gray.500">占用保证金 ≈ {mgn} USDT</Text>
        <Text fontSize="xs" color="gray.500" mb={3}>可用 {fmt(usdt?.available || 0, 2)} USDT</Text>
        <Button w="full" colorScheme={side === "buy" ? "green" : "red"} isLoading={place.isPending} onClick={() => place.mutate()} isDisabled={!token}>
          {side === "buy" ? "买入开多" : "卖出开空"}
        </Button>
        <Box mt={4}>
          <Text fontSize="xs" color="gray.500" mb={1}>订单簿</Text>
          {(bookQ.data?.asks || []).slice().reverse().slice(0, 8).map((l) => (
            <Flex key={"a"+l.price} fontSize="xs" justify="space-between" color="down.500"><span>{fmtUsd(l.price)}</span><span>{fmt(l.qty, 3)}</span></Flex>
          ))}
          <Text textAlign="center" my={1} fontWeight="700">{fmtUsd(mark)}</Text>
          {(bookQ.data?.bids || []).slice(0, 8).map((l) => (
            <Flex key={"b"+l.price} fontSize="xs" justify="space-between" color="up.500"><span>{fmtUsd(l.price)}</span><span>{fmt(l.qty, 3)}</span></Flex>
          ))}
        </Box>
      </Box>

      <Box gridColumn={{ xl: "1 / -1" }} bg="bg.800" borderRadius="md" borderWidth="1px" borderColor="whiteAlpha.100">
        <Tabs size="sm" colorScheme="teal">
          <TabList>
            <Tab>仓位</Tab>
            <Tab>委托</Tab>
            <Tab>最新成交</Tab>
          </TabList>
          <TabPanels>
            <TabPanel>
              {(posQ.data || []).length === 0 && <Text color="gray.500">暂无仓位</Text>}
              {(posQ.data || []).map((p) => {
                const pnl = unrealizedPnl(p.side, p.entry_price, p.mark_price, p.qty);
                const liq = liqPrice(p.side, p.entry_price, p.margin, p.qty);
                return (
                  <Flex key={p.id} gap={4} py={2} fontSize="sm" wrap="wrap" align="center" borderBottomWidth="1px" borderColor="whiteAlpha.100">
                    <Text w="90px">{p.symbol}</Text>
                    <Text color={p.side === "long" ? "up.500" : "down.500"}>{p.side} {p.leverage}x</Text>
                    <Text>数量 {fmt(p.qty, 4)}</Text>
                    <Text>开仓 {fmtUsd(p.entry_price)}</Text>
                    <Text>强平 {fmtUsd(liq.toString())}</Text>
                    <Text color={pnl.gte(0) ? "up.500" : "down.500"}>PnL {fmt(pnl.toString(), 2)}</Text>
                    <Button size="xs" onClick={() => close.mutate(p.id)}>平仓</Button>
                  </Flex>
                );
              })}
            </TabPanel>
            <TabPanel>
              {(ordQ.data || []).slice(0, 12).map((o) => (
                <Flex key={o.id} gap={3} fontSize="sm" py={1}><Text>{o.symbol}</Text><Text>{o.side}</Text><Text>{fmtUsd(o.price)}</Text><Text>{o.qty}</Text><Text color="gray.500">{o.status}</Text></Flex>
              ))}
            </TabPanel>
            <TabPanel>
              {(tradesQ.data || []).slice(0, 16).map((t) => (
                <Flex key={t.id} justify="space-between" fontSize="xs" color={t.side === "buy" ? "up.500" : "down.500"}>
                  <span>{fmtUsd(t.price)}</span><span>{fmt(t.qty, 4)}</span>
                </Flex>
              ))}
            </TabPanel>
          </TabPanels>
        </Tabs>
      </Box>
    </Grid>
  );
}
