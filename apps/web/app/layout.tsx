import type {Metadata} from "next";
import "./globals.css";
export const metadata:Metadata={title:"Inspectly | Property inspections",description:"Property inspections, punch lists and team management."};
export default function RootLayout({children}:{children:React.ReactNode}){return <html lang="en"><body>{children}</body></html>;}
