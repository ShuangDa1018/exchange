import { Box, Heading, Text } from "@chakra-ui/react";

export default function Maintenance() {
  return (
    <Box minH="100vh" display="grid" placeItems="center">
      <Box textAlign="center">
        <Heading size="lg">系统维护中</Heading>
        <Text color="gray.400" mt={3}>TF DEX 正在升级撮合与风控，请稍后再来。</Text>
      </Box>
    </Box>
  );
}
