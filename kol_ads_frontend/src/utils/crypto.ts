import JSEncrypt from 'jsencrypt';

/**
 * 极客级 RSA 加密器
 * @param plaintext 原始密码
 * @param publicKey 后端下发的公钥
 * @returns 加密后的 Base64 密文
 */
export const encryptPassword = (plaintext: string, publicKey: string): string => {
    const encryptor = new JSEncrypt();
    encryptor.setPublicKey(publicKey);

    // 💡 极客防御：加入时间戳防重放攻击 (Replay Attack)
    // 拼装格式："真实密码|1672502400000"
    const timestamp = Date.now();
    const securePayload = `${plaintext}|${timestamp}`;

    const ciphertext = encryptor.encrypt(securePayload);
    return ciphertext ? ciphertext.toString() : '';
};