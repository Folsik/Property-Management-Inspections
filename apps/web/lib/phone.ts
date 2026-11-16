const trunkPrefixCodes = new Set([
 "+44","+33","+49","+34","+31","+32","+41","+43","+45","+46","+47","+48","+61","+64",
 "+81","+82","+86","+91","+90","+20","+27","+234","+254","+251","+92","+880","+94",
 "+60","+62","+63","+66","+84","+353","+358","+351","+972","+966","+971","+974",
 "+7","+380","+381","+385","+386","+387","+389","+420","+421","+423","+40","+36"
]);

// Returns an E.164 telephone number or null. Do not invent missing digits.
export function formatE164(dialCode:string,nationalPhone:string):string|null{
 if(!/^\+[1-9]\d{0,3}$/.test(dialCode))return null;
 if(!/^[0-9\s().-]*$/.test(nationalPhone))return null;
 let digits=nationalPhone.replace(/[\s().-]/g,"");
 if(trunkPrefixCodes.has(dialCode)&&digits.startsWith("0"))digits=digits.slice(1);
 const combined=dialCode+digits;
 return /^\+[1-9]\d{7,14}$/.test(combined)?combined:null;
}
