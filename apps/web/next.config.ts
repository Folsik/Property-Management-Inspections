import type { NextConfig } from "next";
const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  async headers() {
    return [{source:"/invite/:token",headers:[
      {key:"Referrer-Policy",value:"no-referrer"},
      {key:"Cache-Control",value:"private, no-store"},
      {key:"X-Robots-Tag",value:"noindex, nofollow"},
    ]},{source:"/reset-password",headers:[
      {key:"Referrer-Policy",value:"no-referrer"},
      {key:"Cache-Control",value:"private, no-store"},
      {key:"X-Robots-Tag",value:"noindex, nofollow"},
    ]},{source:"/forgot-password",headers:[
      {key:"Referrer-Policy",value:"no-referrer"},
      {key:"Cache-Control",value:"private, no-store"},
      {key:"X-Robots-Tag",value:"noindex, nofollow"},
    ]}];
  },
};
export default nextConfig;
