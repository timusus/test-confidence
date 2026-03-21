package com.example

import io.mockk.every
import io.mockk.mockk
import org.junit.Test

class TautologyTest {
    private val repo = mockk<UserRepository>()
    private val viewModel = UserViewModel(repo)

    @Test
    fun `tautological - direct match`() {
        val user = User("test")
        every { repo.getUser() } returns user
        val result = viewModel.getUser()
        assertThat(result).isEqualTo(user)
    }

    @Test
    fun `tautological - via assignment`() {
        val expected = User("test")
        every { repo.getUser() } returns expected
        val result = viewModel.getUser()
        assertThat(result).isEqualTo(expected)
    }

    @Test
    fun `not tautological - different value`() {
        every { repo.getUser() } returns User("raw")
        val result = viewModel.getUserName()
        assertThat(result).isEqualTo("processed")
    }

    @Test
    fun `not tautological - no stub`() {
        val result = viewModel.computeSomething()
        assertThat(result).isEqualTo(42)
    }
}
