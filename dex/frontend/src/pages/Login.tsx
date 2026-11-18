import { Box, Button, Divider, FormControl, FormLabel, Heading, Input, Stack, Text, useToast } from "@chakra-ui/react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useDispatch } from "react-redux";
import { Wallet } from "ethers";
import { Keypair } from "@solana/web3.js";
import { api, setToken } from "@/api/client";
import { setAuth } from "@/store";

export default function Login() {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("demo@tf.dex");
  const [password, setPassword] = useState("demo123");
  const [invite, setInvite] = useState("");
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  const nav = useNavigate();
  const disp = useDispatch();

  async function done(token: string, user: any) {
    setToken(token);
    disp(setAuth({ token, user }));
    nav("/");
  }

  async function submit() {
    setBusy(true);
    try {
      const r = mode === "login"
        ? await api.login(email, password)
        : await api.register(email, password, email.split("@")[0], invite);
      await done(r.token, r.user);
    } catch (e: any) {
      toast({ status: "error", title: e.message });
    } finally {
      setBusy(false);
    }
  }

  async function evm() {
    setBusy(true);
    try {
      const w = Wallet.createRandom();
      const r = await api.wallet(w.address, "evm", invite);
      await done(r.token, r.user);
    } catch (e: any) {
      toast({ status: "error", title: e.message });
    } finally { setBusy(false); }
  }

  async function sol() {
    setBusy(true);
    try {
      const k = Keypair.generate();
      const r = await api.wallet(k.publicKey.toBase58(), "sol", invite);
      await done(r.token, r.user);
    } catch (e: any) {
      toast({ status: "error", title: e.message });
    } finally { setBusy(false); }
  }

  return (
    <Box minH="100vh" display="grid" placeItems="center" px={4}>
      <Box w="full" maxW="420px" bg="bg.800" p={8} borderRadius="xl" borderWidth="1px" borderColor="whiteAlpha.100">
        <Heading size="lg" mb={1}>TF DEX</Heading>
        <Text color="gray.400" fontSize="sm" mb={6}>永续合约 · 混合清算 · 2026</Text>
        <Stack spacing={4}>
          <FormControl>
            <FormLabel>Email</FormLabel>
            <Input value={email} onChange={(e) => setEmail(e.target.value)} />
          </FormControl>
          <FormControl>
            <FormLabel>Password</FormLabel>
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </FormControl>
          {mode === "register" && (
            <FormControl>
              <FormLabel>邀请码</FormLabel>
              <Input value={invite} onChange={(e) => setInvite(e.target.value)} placeholder="TFDEMO" />
            </FormControl>
          )}
          <Button colorScheme="teal" isLoading={busy} onClick={submit}>{mode === "login" ? "登录" : "注册并领取 10000 USDT"}</Button>
          <Button variant="ghost" onClick={() => setMode(mode === "login" ? "register" : "login")}>
            {mode === "login" ? "没有账号？注册" : "已有账号？登录"}
          </Button>
          <Divider />
          <Text fontSize="xs" color="gray.500">钱包登录（演示：生成本地密钥，不上链）</Text>
          <Button variant="outline" onClick={evm} isLoading={busy}>EVM Wallet</Button>
          <Button variant="outline" onClick={sol} isLoading={busy}>Solana Wallet</Button>
          <Text fontSize="xs" color="gray.600">演示账号 demo@tf.dex / demo123 · 管理 admin@tf.dex / admin123</Text>
        </Stack>
      </Box>
    </Box>
  );
}
