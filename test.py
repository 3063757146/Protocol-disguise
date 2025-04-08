from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad
from Crypto.Random import get_random_bytes

# 待加密的明文
plaintext = "Hello, AES encryption!"
# 密钥长度必须为 16、24 或 32 字节，分别对应 AES-128、AES-192 和 AES-256
key = get_random_bytes(16)

# 加密函数
def encrypt(plaintext, key, mode):
    if mode == AES.MODE_ECB:
        cipher = AES.new(key, mode)
    elif mode in [AES.MODE_CBC, AES.MODE_CFB, AES.MODE_OFB]:
        iv = get_random_bytes(AES.block_size)
        cipher = AES.new(key, mode, iv)
    else:
        raise ValueError("Unsupported mode")

    padded_plaintext = pad(plaintext.encode(), AES.block_size)
    if mode == AES.MODE_ECB:
        ciphertext = cipher.encrypt(padded_plaintext)
        return ciphertext.hex()
    else:
        ciphertext = cipher.encrypt(padded_plaintext)
        return iv.hex() + ciphertext.hex()

# 解密函数
def decrypt(ciphertext, key, mode):
    if mode == AES.MODE_ECB:
        cipher = AES.new(key, mode)
        ciphertext_bytes = bytes.fromhex(ciphertext)
    elif mode in [AES.MODE_CBC, AES.MODE_CFB, AES.MODE_OFB]:
        iv = bytes.fromhex(ciphertext[:32])
        ciphertext_bytes = bytes.fromhex(ciphertext[32:])
        cipher = AES.new(key, mode, iv)
    else:
        raise ValueError("Unsupported mode")

    decrypted_data = cipher.decrypt(ciphertext_bytes)
    unpadded_data = unpad(decrypted_data, AES.block_size)
    return unpadded_data.decode()

# 示例：ECB 模式
ecb_ciphertext = encrypt(plaintext, key, AES.MODE_ECB)
print(f"ECB 模式加密结果: {ecb_ciphertext}")
ecb_decrypted = decrypt(ecb_ciphertext, key, AES.MODE_ECB)
print(f"ECB 模式解密结果: {ecb_decrypted}")

# 示例：CBC 模式
cbc_ciphertext = encrypt(plaintext, key, AES.MODE_CBC)
print(f"CBC 模式加密结果: {cbc_ciphertext}")
cbc_decrypted = decrypt(cbc_ciphertext, key, AES.MODE_CBC)
print(f"CBC 模式解密结果: {cbc_decrypted}")

# 示例：CFB 模式
cfb_ciphertext = encrypt(plaintext, key, AES.MODE_CFB)
print(f"CFB 模式加密结果: {cfb_ciphertext}")
cfb_decrypted = decrypt(cfb_ciphertext, key, AES.MODE_CFB)
print(f"CFB 模式解密结果: {cfb_decrypted}")

# 示例：OFB 模式
ofb_ciphertext = encrypt(plaintext, key, AES.MODE_OFB)
print(f"OFB 模式加密结果: {ofb_ciphertext}")
ofb_decrypted = decrypt(ofb_ciphertext, key, AES.MODE_OFB)
print(f"OFB 模式解密结果: {ofb_decrypted}")    